package relay

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	// Version is the relay config and control protocol version.
	Version = 1
	// DefaultPort is the relay control/data port.
	DefaultPort = 7443
	// ConfigFileName is the relay config file name.
	ConfigFileName = "relay.json"
	// ProfileFileName is the host profile file name.
	ProfileFileName = "relay-profile.json"
	// KeyFileName is the relay identity key file name.
	KeyFileName = "relay.key"
)

// Config is the relay server configuration.
type Config struct {
	Version       int    `json:"version"`
	Listen        string `json:"listen"`
	PublicAddress string `json:"publicAddress"`
	KeyFile       string `json:"keyFile"`
	PasswordSalt  string `json:"passwordSalt"`
	PasswordHash  string `json:"passwordHash"`
	MaxSessions   int    `json:"maxSessions,omitempty"`
	MaxFriends    int    `json:"maxFriends,omitempty"`
}

// HostProfile is the secret handoff file for the host app.
type HostProfile struct {
	Version     int    `json:"version"`
	Address     string `json:"address"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Fingerprint string `json:"fingerprint"`
	Password    string `json:"password"`
}

// HostPort returns the profile's separate host and port, falling back to the
// canonical Address for profiles created before the fields were split.
func (p HostProfile) HostPort() (string, int, error) {
	if p.Host != "" && p.Port > 0 {
		return p.Host, p.Port, nil
	}
	if p.Address == "" {
		return "", 0, errors.New("host profile has no relay address")
	}
	return hostPortFromAddress(p.Address)
}

// DecodeHostProfile parses the owner-only handoff file written by relay setup.
// It returns the canonical separate host and port and a lowercase Relay ID.
func DecodeHostProfile(data []byte) (HostProfile, error) {
	var profile HostProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return HostProfile{}, fmt.Errorf("parse host profile: %w", err)
	}
	if profile.Version != Version {
		return HostProfile{}, fmt.Errorf("host profile version %d is not supported", profile.Version)
	}
	host, port, err := profile.HostPort()
	if err != nil {
		return HostProfile{}, err
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return HostProfile{}, errors.New("the host profile is missing the relay host")
	}
	if port < 1 || port > 65535 {
		return HostProfile{}, errors.New("the host profile has a bad relay port")
	}
	fingerprint := strings.ToLower(strings.TrimSpace(profile.Fingerprint))
	if len(fingerprint) != 64 {
		return HostProfile{}, errors.New("the host profile has a bad Relay ID")
	}
	if _, err := hex.DecodeString(fingerprint); err != nil {
		return HostProfile{}, errors.New("the host profile has a bad Relay ID")
	}
	password := strings.TrimSpace(profile.Password)
	if password == "" {
		return HostProfile{}, errors.New("the host profile is missing the relay password")
	}
	profile.Host = host
	profile.Port = port
	profile.Address = net.JoinHostPort(host, strconv.Itoa(port))
	profile.Fingerprint = fingerprint
	profile.Password = password
	return profile, nil
}

// SetupOptions are the inputs to a first-run relay setup.
type SetupOptions struct {
	Dir           string
	PublicAddress string
	PublicHost    string
	PublicPort    int
	Listen        string
	Password      string
}

// ConfigPath returns the relay config path inside dir.
func ConfigPath(dir string) string { return filepath.Join(dir, ConfigFileName) }

// ProfilePath returns the host profile path inside dir.
func ProfilePath(dir string) string { return filepath.Join(dir, ProfileFileName) }

// Setup creates the relay key, config and host profile.
func Setup(options SetupOptions) (HostProfile, error) {
	dir := strings.TrimSpace(options.Dir)
	if dir == "" {
		return HostProfile{}, errors.New("a relay data directory is required")
	}
	public, err := publicAddressFor(options.PublicHost, options.PublicPort, options.PublicAddress)
	if err != nil {
		return HostProfile{}, err
	}
	host, port, err := hostPortFromAddress(public)
	if err != nil {
		return HostProfile{}, err
	}
	// The public port can differ from the local listen port when Docker or a
	// firewall maps one to the other. The default local port stays 7443.
	listen := strings.TrimSpace(options.Listen)
	if listen == "" {
		listen = ":" + strconv.Itoa(DefaultPort)
	}
	if err := validateListen(listen); err != nil {
		return HostProfile{}, err
	}
	_, statErr := os.Stat(dir)
	createdDir := errors.Is(statErr, os.ErrNotExist)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return HostProfile{}, fmt.Errorf("create relay directory: %w", err)
	}
	if createdDir {
		_ = os.Chmod(dir, 0o700)
	}

	configPath := ConfigPath(dir)
	profilePath := ProfilePath(dir)
	keyPath := filepath.Join(dir, KeyFileName)
	if _, err := os.Stat(configPath); err == nil {
		return HostProfile{}, fmt.Errorf("relay is already configured at %s", configPath)
	}
	if _, err := os.Stat(profilePath); err == nil {
		return HostProfile{}, fmt.Errorf("host profile already exists at %s", profilePath)
	}
	if _, err := os.Stat(keyPath); err == nil {
		return HostProfile{}, fmt.Errorf("relay key already exists at %s", keyPath)
	}

	password := strings.TrimSpace(options.Password)
	if password == "" {
		password, err = GeneratePassword()
		if err != nil {
			return HostProfile{}, err
		}
	}
	salt, hash, err := HashPassword(password)
	if err != nil {
		return HostProfile{}, err
	}
	_, fingerprint, err := GenerateIdentity(keyPath)
	if err != nil {
		return HostProfile{}, err
	}

	cfg := Config{
		Version:       Version,
		Listen:        listen,
		PublicAddress: public,
		KeyFile:       KeyFileName,
		PasswordSalt:  salt,
		PasswordHash:  hash,
		MaxSessions:   16,
		MaxFriends:    64,
	}
	if err := SaveConfig(configPath, cfg); err != nil {
		return HostProfile{}, err
	}
	profile := HostProfile{
		Version:     Version,
		Address:     public,
		Host:        host,
		Port:        port,
		Fingerprint: fingerprint,
		Password:    password,
	}
	if err := SaveProfile(profilePath, profile); err != nil {
		return HostProfile{}, err
	}
	return profile, nil
}

// LoadConfig reads and validates the relay config.
func LoadConfig(path string) (Config, error) {
	if err := checkSecretFile(path); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read relay config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse relay config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SaveConfig writes the relay config owner-only.
func SaveConfig(path string, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create relay config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode relay config: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write relay config: %w", err)
	}
	return os.Chmod(path, 0o600)
}

// SaveProfile writes the host profile owner-only.
func SaveProfile(path string, profile HostProfile) error {
	if profile.Version != Version || profile.Address == "" || profile.Fingerprint == "" || profile.Password == "" {
		return errors.New("host profile is incomplete")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode host profile: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write host profile: %w", err)
	}
	return os.Chmod(path, 0o600)
}

// SetPassword changes the relay password without rotating the identity. The
// relay must be restarted and the host profile re-imported.
func SetPassword(dir, password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return errors.New("a relay password is required")
	}
	configPath := ConfigPath(dir)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	salt, hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	cfg.PasswordSalt = salt
	cfg.PasswordHash = hash
	if err := SaveConfig(configPath, cfg); err != nil {
		return err
	}

	profilePath := ProfilePath(dir)
	data, err := os.ReadFile(profilePath)
	switch {
	case err == nil:
		var profile HostProfile
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("parse host profile: %w", err)
		}
		if profile.Host == "" || profile.Port == 0 {
			if host, port, hostPortErr := profile.HostPort(); hostPortErr == nil {
				profile.Host, profile.Port = host, port
			}
		}
		profile.Password = password
		if err := SaveProfile(profilePath, profile); err != nil {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		_, fingerprint, idErr := LoadIdentity(KeyPath(configPath, cfg))
		if idErr != nil {
			return idErr
		}
		host, port, hostPortErr := hostPortFromAddress(cfg.PublicAddress)
		if hostPortErr != nil {
			return hostPortErr
		}
		profile := HostProfile{
			Version:     Version,
			Address:     cfg.PublicAddress,
			Host:        host,
			Port:        port,
			Fingerprint: fingerprint,
			Password:    password,
		}
		if err := SaveProfile(profilePath, profile); err != nil {
			return err
		}
	default:
		return fmt.Errorf("read host profile: %w", err)
	}
	return nil
}

// KeyPath resolves the relay key path relative to its config file.
func KeyPath(configPath string, cfg Config) string {
	if filepath.IsAbs(cfg.KeyFile) {
		return cfg.KeyFile
	}
	return filepath.Join(filepath.Dir(configPath), cfg.KeyFile)
}

func (cfg Config) validate() error {
	if cfg.Version != Version {
		return fmt.Errorf("relay config version %d is not supported", cfg.Version)
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return errors.New("relay config has no listen address")
	}
	if err := validateListen(cfg.Listen); err != nil {
		return err
	}
	if _, _, err := normalizePublicAddress(cfg.PublicAddress); err != nil {
		return err
	}
	if cfg.KeyFile == "" {
		return errors.New("relay config has no key file")
	}
	if _, err := base64.StdEncoding.DecodeString(cfg.PasswordSalt); err != nil {
		return errors.New("relay config has a bad password salt")
	}
	if _, err := base64.StdEncoding.DecodeString(cfg.PasswordHash); err != nil {
		return errors.New("relay config has a bad password hash")
	}
	if cfg.MaxSessions < 0 || cfg.MaxFriends < 0 {
		return errors.New("relay limits cannot be negative")
	}
	return nil
}

func publicAddressFor(host string, port int, address string) (string, error) {
	host = strings.TrimSpace(host)
	if host != "" {
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
		if port == 0 {
			port = DefaultPort
		}
		if port < 1 || port > 65535 {
			return "", errors.New("the public relay port must be between 1 and 65535")
		}
		if strings.ContainsAny(host, " /") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
			return "", errors.New("the public relay host is invalid")
		}
		return net.JoinHostPort(host, strconv.Itoa(port)), nil
	}
	value, _, err := normalizePublicAddress(address)
	return value, err
}

func hostPortFromAddress(address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("public relay address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, errors.New("the public relay port must be between 1 and 65535")
	}
	return host, port, nil
}

func normalizePublicAddress(value string) (address string, port int, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0, errors.New("a public relay address is required")
	}
	if host, portText, splitErr := net.SplitHostPort(value); splitErr == nil {
		if strings.TrimSpace(host) == "" {
			return "", 0, errors.New("the public relay address is missing a host")
		}
		port, err = strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", 0, errors.New("the public relay port must be between 1 and 65535")
		}
		return net.JoinHostPort(host, strconv.Itoa(port)), port, nil
	}
	if ip := net.ParseIP(value); ip != nil {
		return net.JoinHostPort(ip.String(), strconv.Itoa(DefaultPort)), DefaultPort, nil
	}
	if strings.ContainsAny(value, " /:") {
		return "", 0, errors.New("use host:port for the public relay address")
	}
	return net.JoinHostPort(value, strconv.Itoa(DefaultPort)), DefaultPort, nil
}

func checkSecretFile(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("check secret file %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s is accessible by other users; run chmod 600 %s", path, path)
	}
	return nil
}

func validateListen(value string) error {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return errors.New("the relay listen address must be host:port, for example :7443")
	}
	if strings.ContainsAny(host, " /") {
		return errors.New("the relay listen address is invalid")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("the relay listen port must be between 1 and 65535")
	}
	return nil
}
