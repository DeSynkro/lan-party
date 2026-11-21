package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/relay"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "setup", "init":
		runSetup(os.Args[2:])
	case "run":
		runRelay(os.Args[2:])
	case "relay-id", "fingerprint":
		runFingerprint(os.Args[2:])
	case "passwd":
		runPasswd(os.Args[2:])
	case "rotate-key":
		runRotateKey(os.Args[2:])
	case "show-profile":
		runShowProfile(os.Args[2:])
	case "show-info":
		runShowInfo(os.Args[2:])
	case "version", "--version":
		fmt.Println(buildinfo.Version)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: lan-party-relay <command>

Commands:
  setup          Create the relay key, config and host profile
  run            Run the relay
  relay-id       Print the Relay ID (public-key fingerprint)
  fingerprint    Alias for relay-id
  passwd         Change the relay password without rotating the key
  rotate-key     Replace the relay key and update the host profile fingerprint
  show-info      Print the address, fingerprint and password for the host app
  show-profile   Print the host profile as JSON
  version        Print the relay version

The relay reads LANPARTY_RELAY_CONFIG when --config is not given.`)
}

func configDefault() string {
	if value := strings.TrimSpace(os.Getenv("LANPARTY_RELAY_CONFIG")); value != "" {
		return value
	}
	return relay.ConfigFileName
}

func runSetup(args []string) {
	flags := flag.NewFlagSet("setup", flag.ExitOnError)
	dir := flags.String("dir", workingDir(), "relay data directory")
	address := flags.String("address", "", "public relay address as host or host:port")
	listen := flags.String("listen", "", "local listen address as host:port")
	password := flags.String("password", "", "relay password (or LANPARTY_RELAY_PASSWORD)")
	nonInteractive := flags.Bool("non-interactive", false, "do not prompt")
	flags.Parse(args)

	if *password == "" {
		*password = os.Getenv("LANPARTY_RELAY_PASSWORD")
	}
	*address = strings.TrimSpace(*address)
	for *address == "" {
		if *nonInteractive {
			log.Fatal("setup: --address is required in non-interactive mode")
		}
		answer, err := prompt("Public relay address (host or host:port, default port 7443): ")
		if err != nil {
			log.Fatal(err)
		}
		*address = answer
	}

	profile, err := relay.Setup(relay.SetupOptions{
		Dir:           *dir,
		PublicAddress: *address,
		Listen:        *listen,
		Password:      *password,
	})
	if err != nil {
		log.Fatal(err)
	}
	configPath := relay.ConfigPath(*dir)
	profilePath := relay.ProfilePath(*dir)
	fmt.Printf(`Relay configured in %s

Public address: %s
Fingerprint:    %s
Host profile:   %s

The host profile contains the relay password. Keep it private.
Run "lan-party-relay show-info --dir %s" to display the values for the host app.

Start the relay with:
  lan-party-relay run --config %s
`, *dir, profile.Address, profile.Fingerprint, profilePath, *dir, configPath)
}

func runRelay(args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := flags.String("config", configDefault(), "relay config file")
	flags.Parse(args)

	cfg, err := relay.LoadConfig(*configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
		}
		public := strings.TrimSpace(os.Getenv("LANPARTY_RELAY_PUBLIC_ADDRESS"))
		if public == "" {
			log.Fatalf("relay is not configured at %s; set LANPARTY_RELAY_PUBLIC_ADDRESS to create it automatically, or run `lan-party-relay setup --dir %s`",
				*configPath, filepath.Dir(*configPath))
		}
		profile, setupErr := relay.Setup(relay.SetupOptions{
			Dir:           filepath.Dir(*configPath),
			PublicAddress: public,
			Password:      os.Getenv("LANPARTY_RELAY_PASSWORD"),
		})
		if setupErr != nil {
			log.Fatalf("relay auto-setup: %v", setupErr)
		}
		log.Printf("relay: created config %s", *configPath)
		log.Printf("relay: host profile %s", relay.ProfilePath(filepath.Dir(*configPath)))
		log.Printf("relay: fingerprint %s", profile.Fingerprint)
		cfg, err = relay.LoadConfig(*configPath)
		if err != nil {
			log.Fatal(err)
		}
	}
	cert, fingerprint, err := relay.LoadIdentity(relay.KeyPath(*configPath, cfg))
	if err != nil {
		log.Fatal(err)
	}
	server, err := relay.NewServer(cfg, cert, fingerprint)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	log.Printf("relay: version %s", buildinfo.Version)
	log.Printf("relay: listening on %s", cfg.Listen)
	log.Printf("relay: public address %s", cfg.PublicAddress)
	log.Printf("relay: fingerprint %s", fingerprint)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}

func runFingerprint(args []string) {
	flags := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	configPath := flags.String("config", configDefault(), "relay config file")
	flags.Parse(args)

	cfg, err := relay.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	_, fingerprint, err := relay.LoadIdentity(relay.KeyPath(*configPath, cfg))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(fingerprint)
}

func runPasswd(args []string) {
	flags := flag.NewFlagSet("passwd", flag.ExitOnError)
	dir := flags.String("dir", workingDir(), "relay data directory")
	password := flags.String("password", "", "new relay password (or LANPARTY_RELAY_PASSWORD)")
	passwordStdin := flags.Bool("password-stdin", false, "read the new password from stdin")
	flags.Parse(args)

	if *passwordStdin && *password != "" {
		log.Fatal("passwd: use --password-stdin or --password, not both")
	}
	if *passwordStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			log.Fatal(err)
		}
		*password = strings.TrimRight(line, "\r\n")
	} else if *password == "" {
		*password = os.Getenv("LANPARTY_RELAY_PASSWORD")
	}
	if strings.TrimSpace(*password) == "" {
		log.Fatal("passwd: set --password, --password-stdin or LANPARTY_RELAY_PASSWORD")
	}
	if err := relay.SetPassword(*dir, *password); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Relay password updated. Restart the relay and re-import the host profile.")
}

func runRotateKey(args []string) {
	flags := flag.NewFlagSet("rotate-key", flag.ExitOnError)
	dir := flags.String("dir", workingDir(), "relay data directory")
	flags.Parse(args)

	fingerprint, err := relay.RotateIdentity(*dir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Relay identity rotated. New fingerprint: %s\n\nRestart the relay and re-import the updated host profile.\n", fingerprint)
}

func runShowProfile(args []string) {
	flags := flag.NewFlagSet("show-profile", flag.ExitOnError)
	dir := flags.String("dir", workingDir(), "relay data directory")
	flags.Parse(args)

	data, err := os.ReadFile(relay.ProfilePath(*dir))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		log.Fatal(err)
	}
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func runShowInfo(args []string) {
	flags := flag.NewFlagSet("show-info", flag.ExitOnError)
	dir := flags.String("dir", workingDir(), "relay data directory")
	flags.Parse(args)

	data, err := os.ReadFile(relay.ProfilePath(*dir))
	if err != nil {
		log.Fatal(err)
	}
	var profile relay.HostProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Relay address:     %s\n", profile.Address)
	fmt.Printf("Relay ID:          %s\n", profile.Fingerprint)
	fmt.Printf("Relay password:    %s\n", profile.Password)
	fmt.Println("\nEnter these in LAN Party under Settings > Relay.")
	fmt.Println("The relay is host-only: keep this terminal private, and share invites instead of the Relay ID or password.")
}

func prompt(label string) (string, error) {
	fmt.Print(label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
