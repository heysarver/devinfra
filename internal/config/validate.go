package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)
var singleCharRegex = regexp.MustCompile(`^[a-z]$`)

var reservedNames = map[string]bool{
	"traefik":      true,
	"dnsmasq":      true,
	"infra":        true,
	"test":         true,
	"default":      true,
	"socket-proxy": true,
	"devinfra":     true,
	"di":           true,
	"all":          true,
	"localhost":    true,
}

// ValidateName checks that a project name is a valid RFC 1123 DNS label.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("name must be 63 characters or fewer (DNS label limit)")
	}
	if !nameRegex.MatchString(name) && !singleCharRegex.MatchString(name) {
		return fmt.Errorf("name must be lowercase alphanumeric with hyphens, start with a letter, not end with hyphen")
	}
	if reservedNames[name] {
		return fmt.Errorf("name %q is reserved", name)
	}
	return nil
}

var reservedPorts = map[int]bool{
	80:   true,
	443:  true,
	5354: true,
}

// ValidatePort checks that a port number is valid and not reserved.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if reservedPorts[port] {
		return fmt.Errorf("port %d is reserved by dev-infra", port)
	}
	return nil
}

// ParsePort parses a port string into an integer and validates it.
func ParsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("port must be an integer: %s", s)
	}
	if err := ValidatePort(port); err != nil {
		return 0, err
	}
	return port, nil
}

// AppIndicatorFiles are filenames whose presence suggests a directory
// already contains an application project.
var AppIndicatorFiles = []string{
	"docker-compose.yaml", "docker-compose.yml", "compose.yaml", "compose.yml",
	"package.json", "go.mod", "Gemfile", "requirements.txt", "pyproject.toml",
	"Cargo.toml", "pom.xml", "build.gradle", "mix.exs", "pubspec.yaml",
	"CMakeLists.txt", "setup.py", "setup.cfg",
}

// FindAppIndicator checks whether dir contains any app indicator file.
// Returns the first matching filename and true, or ("", false) if none found.
func FindAppIndicator(dir string) (string, bool) {
	for _, indicator := range AppIndicatorFiles {
		if _, err := os.Stat(filepath.Join(dir, indicator)); err == nil {
			return indicator, true
		}
	}
	return "", false
}

// SensitiveDirs are system directories where projects must not be created.
var SensitiveDirs = []string{"/etc", "/usr", "/var", "/tmp", "/bin", "/sbin"}
