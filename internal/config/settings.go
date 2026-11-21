package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultAppPort is the UDP port the host listens on unless it is changed.
const DefaultAppPort = 51820

// DefaultRelayPort is the public TCP and UDP port of a relay.
const DefaultRelayPort = 7443

// Settings are machine-level preferences. They are not secret and do not
// include group membership.
type Settings struct {
	CloseToTray       bool   `json:"closeToTray"`
	StartOnLogin      bool   `json:"startOnLogin"`
	StartMinimised    bool   `json:"startMinimised"`
	AutoReconnect     bool   `json:"autoReconnect"`
	ApproveNewDevices bool   `json:"approveNewDevices"`
	PublicAddress     string `json:"publicAddress"`
	AppPort           int    `json:"appPort"`
	UseRelay          bool   `json:"useRelay,omitempty"`
	RelayHost         string `json:"relayHost,omitempty"`
	RelayPort         int    `json:"relayPort,omitempty"`
	RelayID           string `json:"relayID,omitempty"`
}

// DefaultSettings returns the first-run values.
func DefaultSettings() Settings {
	return Settings{
		CloseToTray:       true,
		AutoReconnect:     true,
		ApproveNewDevices: true,
		AppPort:           DefaultAppPort,
		RelayPort:         DefaultRelayPort,
	}
}

// LoadSettings reads settings.json. A missing file is not an error.
func LoadSettings(p Paths) (Settings, error) {
	s := DefaultSettings()
	path := filepath.Join(p.ConfigDir, "settings.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings(), fmt.Errorf("parse settings: %w", err)
	}
	if s.AppPort == 0 {
		s.AppPort = DefaultAppPort
	}
	if s.RelayPort == 0 {
		s.RelayPort = DefaultRelayPort
	}
	return s, nil
}

// SaveSettings writes settings.json atomically with owner-only permissions.
func SaveSettings(p Paths, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(p.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	tmp := filepath.Join(p.ConfigDir, "settings.json.tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(p.ConfigDir, "settings.json")); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

// Validate checks the values that leave the app: the local listening port and
// the address friends will be told to connect to.
func (s Settings) Validate() error {
	if s.AppPort < 1 || s.AppPort > 65535 {
		return errors.New("the app port must be between 1 and 65535")
	}
	if err := ValidateAddress(s.PublicAddress); err != nil {
		return err
	}
	if s.UseRelay || strings.TrimSpace(s.RelayHost) != "" || strings.TrimSpace(s.RelayID) != "" {
		if err := validateRelayHost(s.RelayHost); err != nil {
			return err
		}
		if s.RelayPort < 1 || s.RelayPort > 65535 {
			return errors.New("the relay port must be between 1 and 65535")
		}
		if len(s.RelayID) != 64 {
			return errors.New("the Relay ID must be 64 hex characters")
		}
		if _, err := hex.DecodeString(s.RelayID); err != nil {
			return errors.New("the Relay ID must be 64 hex characters")
		}
	}
	return nil
}

func validateRelayHost(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("the relay host is empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return errors.New("the relay host cannot contain spaces")
	}
	if strings.Contains(value, "://") {
		return errors.New("use a hostname or address, not a URL")
	}
	if ip := net.ParseIP(strings.Trim(value, "[]")); ip != nil {
		return nil
	}
	if strings.Contains(value, ":") {
		return errors.New("store the relay port separately from the relay host")
	}
	return validateHost(value)
}

// ValidateAddress accepts an empty value, a hostname, an IP address, or a
// host:port pair. It rejects URLs and anything with spaces.
func ValidateAddress(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return errors.New("the public address cannot contain spaces")
	}
	if strings.Contains(s, "://") {
		return errors.New("use a hostname or address, not a URL")
	}
	if host, port, err := net.SplitHostPort(s); err == nil {
		if host == "" {
			return errors.New("the public address is missing a host")
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("the public address port must be between 1 and 65535")
		}
		return validateHost(host)
	}
	if ip := net.ParseIP(s); ip != nil {
		return nil
	}
	return validateHost(s)
}

func validateHost(host string) error {
	if host == "" {
		return errors.New("the public address is empty")
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	if strings.ContainsAny(host, " /:") {
		return errors.New("use a hostname like home.example.net or an IP address")
	}
	return nil
}
