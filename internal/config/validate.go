package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

// tldLabelRe matches a valid DNS label: lowercase alphanumeric, hyphens allowed
// in the middle. Same rules as RFC 1123.
var tldLabelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// commonPublicTLDs is a short list of real TLDs that would cause confusion.
var commonPublicTLDs = map[string]bool{
	"com": true, "net": true, "org": true, "io": true,
	"dev": true, "app": true, "gov": true, "edu": true,
}

// domainRe matches a valid DNS domain: labels separated by dots, each label
// is lowercase alphanumeric with optional interior hyphens.
var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// ValidateRemoteDomain checks that domain is a valid multi-label DNS name
// (e.g. "claw.sarvent.cloud"). Must have at least two labels, no trailing dot.
func ValidateRemoteDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("remote domain cannot be empty")
	}
	if strings.Contains(domain, "..") {
		return fmt.Errorf("remote domain %q is invalid: consecutive dots are not allowed", domain)
	}
	if strings.HasSuffix(domain, ".") {
		return fmt.Errorf("remote domain %q is invalid: trailing dot not allowed", domain)
	}
	if !domainRe.MatchString(domain) {
		return fmt.Errorf("remote domain %q is invalid: must be a valid DNS name (e.g. claw.sarvent.cloud)", domain)
	}
	return nil
}

// ValidateACMEEmail checks that email looks like a basic email address.
func ValidateACMEEmail(email string) error {
	if email == "" {
		return fmt.Errorf("ACME email cannot be empty")
	}
	at := strings.Index(email, "@")
	if at < 1 || at == len(email)-1 {
		return fmt.Errorf("ACME email %q is invalid: must be in user@domain format", email)
	}
	if strings.Contains(email[at+1:], "@") {
		return fmt.Errorf("ACME email %q is invalid: multiple @ signs", email)
	}
	return nil
}

// ValidateTLD checks that tld is a valid DNS label and emits warnings for
// known problematic values.
func ValidateTLD(tld string) error {
	if tld == "" {
		return fmt.Errorf("TLD cannot be empty")
	}
	if len(tld) > 63 {
		return fmt.Errorf("TLD must be 63 characters or fewer")
	}
	if !tldLabelRe.MatchString(tld) {
		return fmt.Errorf("TLD %q is invalid: must contain only lowercase letters, digits, and hyphens, and must not start or end with a hyphen", tld)
	}
	if tld == "localhost" {
		return fmt.Errorf("TLD 'localhost' is reserved (RFC 6761) and cannot be used")
	}

	// Advisory warnings
	if tld == "local" && runtime.GOOS == "darwin" {
		fmt.Fprintln(os.Stderr, "[WARN] '.local' is used by Bonjour/mDNS on macOS and may cause DNS resolution conflicts.")
	}
	if commonPublicTLDs[tld] {
		fmt.Fprintf(os.Stderr, "[WARN] '.%s' is a real public TLD. Using it locally may break access to real websites on that TLD.\n", tld)
	}

	return nil
}
