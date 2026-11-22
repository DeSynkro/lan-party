// Command lan-party-web runs the LAN Party host from a browser.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	appengine "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/applog"
	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/server/internal/webui"
	"golang.org/x/term"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		if !errors.Is(err, errUsage) {
			fmt.Fprintln(os.Stderr, "lan-party-web:", err)
		}
		os.Exit(1)
	}
}

var errUsage = errors.New("usage")

func dispatch(args []string) error {
	if len(args) == 0 {
		return runServer(nil)
	}
	command, rest := args[0], args[1:]
	switch command {
	case "run":
		return runServer(rest)
	case "setup":
		return runSetup(rest)
	case "passwd":
		return runPasswd(rest)
	case "show-info":
		return runShowInfo(rest)
	case "healthcheck":
		return runHealthcheck(rest)
	case "version":
		fmt.Println(buildinfo.Version)
		return nil
	case "help", "-h", "--help":
		usage()
		return errUsage
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		usage()
		return errUsage
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: lan-party-web <command> [options]

Commands:
  run          Start the web host
  setup        Set the admin password and create the data directory
  passwd       Change the admin password
  show-info    Show the data directory, listen address and app port
  healthcheck  Check a running web host
  version      Print the version

Environment:
  LANPARTY_WEB_DATA_DIR         Data directory (default ./data)
  LANPARTY_WEB_LISTEN           Listen address (default 0.0.0.0:8080)
  LANPARTY_WEB_PORT            Published web UI port (default 8080)
  LANPARTY_APP_PORT            UDP app port (default 51820)
  LANPARTY_WEB_TLS_CERT         TLS certificate file
  LANPARTY_WEB_TLS_KEY          TLS key file
  LANPARTY_WEB_TRUSTED_PROXIES  Comma-separated proxy CIDRs
  LANPARTY_WEB_PASSWORD         First-run password
`)
}

type commonFlags struct {
	dataDir string
	listen  string
	cert    string
	key     string
	appPort int
	webPort int
	proxies string
}

func addCommonFlags(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	fs.StringVar(&c.dataDir, "data-dir", "", "data directory (default ./data)")
	fs.StringVar(&c.listen, "listen", envOr("LANPARTY_WEB_LISTEN", "0.0.0.0:8080"), "listen address")
	fs.StringVar(&c.cert, "tls-cert", os.Getenv("LANPARTY_WEB_TLS_CERT"), "TLS certificate file")
	fs.StringVar(&c.key, "tls-key", os.Getenv("LANPARTY_WEB_TLS_KEY"), "TLS key file")
	fs.IntVar(&c.appPort, "app-port", envIntOr("LANPARTY_APP_PORT", 51820), "UDP app port")
	fs.IntVar(&c.webPort, "web-port", envIntOr("LANPARTY_WEB_PORT", 8080), "published web UI port (shown in Settings)")
	fs.StringVar(&c.proxies, "trusted-proxies", os.Getenv("LANPARTY_WEB_TRUSTED_PROXIES"), "comma-separated proxy CIDRs")
	return c
}

func (c *commonFlags) config() (webui.Config, error) {
	dataDir, err := webui.ResolveDataDir(c.dataDir)
	if err != nil {
		return webui.Config{}, err
	}
	proxies, err := webui.ParseTrustedProxies(c.proxies)
	if err != nil {
		return webui.Config{}, fmt.Errorf("trusted proxies: %w", err)
	}
	return webui.Config{
		DataDir:        dataDir,
		Listen:         c.listen,
		TLSCert:        c.cert,
		TLSKey:         c.key,
		AppPort:        c.appPort,
		WebPort:        c.webPort,
		InContainer:    envBool("LANPARTY_WEB_CONTAINER") || fileExists("/.dockerenv"),
		TrustedProxies: proxies,
	}, nil
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	c := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := c.config()
	if err != nil {
		return err
	}
	if cfg.AppPort < 1 || cfg.AppPort > 65535 {
		return errors.New("the app port must be between 1 and 65535")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	logFile, err := openLog(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()
	_, generated, err := webui.EnsureCredentials(webui.CredentialsPath(cfg.DataDir), strings.TrimSpace(os.Getenv("LANPARTY_WEB_PASSWORD")))
	if err != nil {
		return err
	}
	if generated != "" {
		log.Printf("first-run admin password (shown once): %s", generated)
		fmt.Printf("\nFirst-run admin password (shown once):\n\n  %s\n\nStore it in your password manager, then open the web UI.\n\n", generated)
	}

	engine, err := appengine.NewWithOptions(
		webui.ConfigPaths(cfg.DataDir),
		secure.NewFileStore(filepath.Join(cfg.DataDir, "keys")),
		appengine.Options{DisableAutostart: true, DisableRelay: true},
	)
	if err != nil {
		return err
	}
	defer engine.Close()

	settings := engine.Settings()
	if settings.AppPort != cfg.AppPort {
		settings.AppPort = cfg.AppPort
		if err := engine.SaveSettings(settings); err != nil {
			return fmt.Errorf("set app port: %w", err)
		}
	}

	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		if !isLoopbackListen(cfg.Listen) {
			log.Printf("web: serving plain HTTP on %s; use TLS or a reverse proxy before exposing it", cfg.Listen)
		}
	}

	server, err := webui.New(engine, cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("web: listening on %s", cfg.Listen)
	return server.Run(ctx)
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return false
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	c := addCommonFlags(fs)
	passwordFlag := fs.String("password", "", "admin password (otherwise prompt or generate)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := c.config()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	supplied := strings.TrimSpace(*passwordFlag)
	if supplied == "" {
		supplied = strings.TrimSpace(os.Getenv("LANPARTY_WEB_PASSWORD"))
	}
	if supplied == "" && term.IsTerminal(int(os.Stdin.Fd())) {
		first, err := promptPassword("Choose an admin password: ")
		if err != nil {
			return err
		}
		second, err := promptPassword("Repeat the password: ")
		if err != nil {
			return err
		}
		if first != second {
			return errors.New("the passwords do not match")
		}
		supplied = first
	}
	_, generated, err := webui.EnsureCredentials(webui.CredentialsPath(cfg.DataDir), supplied)
	if err != nil {
		return err
	}
	if generated != "" {
		fmt.Printf("\nFirst-run admin password (shown once):\n\n  %s\n\nStore it in your password manager, then open the web UI.\n", generated)
	} else {
		fmt.Printf("Admin password is set. Data directory: %s\n", cfg.DataDir)
	}
	return nil
}

func runPasswd(args []string) error {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	dataDirFlag := fs.String("data-dir", "", "data directory (default ./data)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dataDir, err := webui.ResolveDataDir(*dataDirFlag)
	if err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("passwd needs an interactive terminal")
	}
	first, err := promptPassword("New admin password: ")
	if err != nil {
		return err
	}
	second, err := promptPassword("Repeat the new password: ")
	if err != nil {
		return err
	}
	if first != second {
		return errors.New("the passwords do not match")
	}
	if err := webui.SetPassword(webui.CredentialsPath(dataDir), first); err != nil {
		return err
	}
	fmt.Println("Password changed. All browser sessions are logged out.")
	return nil
}

func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func runShowInfo(args []string) error {
	fs := flag.NewFlagSet("show-info", flag.ContinueOnError)
	c := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := c.config()
	if err != nil {
		return err
	}
	fmt.Printf("data directory: %s\n", cfg.DataDir)
	fmt.Printf("listen:         %s\n", cfg.Listen)
	fmt.Printf("web port:       %d/tcp\n", cfg.WebPort)
	fmt.Printf("app port:       %d/udp\n", cfg.AppPort)
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		fmt.Println("tls:            enabled")
	} else {
		fmt.Println("tls:            disabled")
	}
	return nil
}

func runHealthcheck(args []string) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := fs.String("url", "", "health endpoint (default http://127.0.0.1:8080/healthz, or https when TLS is configured)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*url)
	if target == "" {
		target = defaultHealthURL(os.Getenv("LANPARTY_WEB_TLS_CERT"), os.Getenv("LANPARTY_WEB_TLS_KEY"))
	}
	client := &http.Client{Timeout: 2 * time.Second}
	if strings.HasPrefix(target, "https://") {
		// The healthcheck talks to loopback with the deployment's own
		// possibly self-signed certificate.
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	resp, err := client.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: %s", resp.Status)
	}
	return nil
}

func defaultHealthURL(certFile, keyFile string) string {
	scheme := "http"
	if certFile != "" && keyFile != "" {
		scheme = "https"
	}
	return scheme + "://127.0.0.1:8080/healthz"
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envIntOr(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func openLog(dataDir string) (*os.File, error) {
	file, err := applog.Open(dataDir)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	return file, nil
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
