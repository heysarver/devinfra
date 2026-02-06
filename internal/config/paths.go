package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/adrg/xdg"
)

// ConfigDir returns the devinfra configuration directory.
// Priority: $DEVINFRA_HOME > $XDG_CONFIG_HOME/devinfra > ~/.config/devinfra
func ConfigDir() string {
	if dir := os.Getenv("DEVINFRA_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(xdg.ConfigHome, "devinfra")
}

func ComposeDir() string    { return filepath.Join(ConfigDir(), "compose") }
func CertsDir() string      { return filepath.Join(ConfigDir(), "certs") }
func DynamicDir() string    { return filepath.Join(ConfigDir(), "dynamic") }
func RegistryPath() string  { return filepath.Join(ConfigDir(), "projects.yaml") }
func EnvFilePath() string   { return filepath.Join(ConfigDir(), ".env") }
func ComposeFile() string   { return filepath.Join(ComposeDir(), "docker-compose.yaml") }
func DnsmasqConf() string   { return filepath.Join(ComposeDir(), "dnsmasq.conf") }

// IsInitialized returns true if the config directory and compose file exist.
func IsInitialized() bool {
	_, err := os.Stat(ComposeFile())
	return err == nil
}

// DNSPort reads DNS_PORT from environment or returns the default.
func DNSPort() string {
	if p := os.Getenv("DNS_PORT"); p != "" {
		return p
	}
	return "5354"
}

// EnsureDirs creates all required directories with appropriate permissions.
// On macOS, it also creates a ~/.config/devinfra symlink pointing to the
// actual config directory under ~/Library/Application Support.
func EnsureDirs() error {
	dirs := []string{
		ConfigDir(),
		ComposeDir(),
		CertsDir(),
		DynamicDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}

	if runtime.GOOS == "darwin" {
		ensureConfigSymlink()
	}

	return nil
}

// ensureConfigSymlink creates a ~/.config/devinfra symlink to the actual
// config directory on macOS where xdg resolves to ~/Library/Application Support.
func ensureConfigSymlink() {
	configDir := ConfigDir()
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dotConfigLink := filepath.Join(home, ".config", "devinfra")

	// Nothing to do if the real config dir is already at ~/.config/devinfra
	if configDir == dotConfigLink {
		return
	}

	// If the symlink already exists and points to the right place, skip
	if target, err := os.Readlink(dotConfigLink); err == nil {
		if target == configDir {
			return
		}
	}

	// Don't clobber a real directory
	if info, err := os.Lstat(dotConfigLink); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return
	}

	// Ensure ~/.config exists
	_ = os.MkdirAll(filepath.Join(home, ".config"), 0755)

	// Remove stale symlink if present, then create
	_ = os.Remove(dotConfigLink)
	_ = os.Symlink(configDir, dotConfigLink)
}
