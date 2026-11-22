package webui

import (
	"net/netip"
	"os"
	"strings"

	"github.com/desynkro/lan-party/internal/config"
)

// Config is the runtime configuration for the web host.
type Config struct {
	DataDir        string
	Listen         string
	TLSCert        string
	TLSKey         string
	AppPort        int
	WebPort        int
	InContainer    bool
	TrustedProxies []netip.Prefix
}

// ResolveDataDir picks the data directory from the flag, the environment, or
// the default beside the binary's working directory.
func ResolveDataDir(flagValue string) (string, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("LANPARTY_WEB_DATA_DIR")); v != "" {
		return v, nil
	}
	return "./data", nil
}

// ConfigPaths keeps the engine's settings and data in one directory.
func ConfigPaths(dataDir string) config.Paths {
	return config.Paths{ConfigDir: dataDir, DataDir: dataDir}
}

// ParseTrustedProxies parses a comma-separated CIDR list.
func ParseTrustedProxies(value string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, err
		}
		out = append(out, prefix)
	}
	return out, nil
}
