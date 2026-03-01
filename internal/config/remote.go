package config

import (
	"os"
	"strings"
)

// RemoteConfig holds configuration for the cross-device remote domain feature.
type RemoteConfig struct {
	Enabled         bool
	Domain          string
	DNSProvider     string
	ACMEEmail       string
	CloudflareToken string
}

// Remote reads the remote domain configuration from the environment and .env file.
// Environment variables take precedence over .env file values.
func Remote() RemoteConfig {
	var r RemoteConfig

	env := readEnvFile()

	r.Enabled = parseBool(getEnvOrFile("REMOTE_ENABLED", env))
	r.Domain = getEnvOrFile("REMOTE_DOMAIN", env)
	r.DNSProvider = getEnvOrFile("REMOTE_DNS_PROVIDER", env)
	r.ACMEEmail = getEnvOrFile("REMOTE_ACME_EMAIL", env)
	r.CloudflareToken = getEnvOrFile("CF_DNS_API_TOKEN", env)

	return r
}

// RemoteEnabled returns true if the remote domain feature is enabled.
func RemoteEnabled() bool {
	return Remote().Enabled
}

// readEnvFile parses the .env file into a key→value map.
func readEnvFile() map[string]string {
	m := make(map[string]string)
	data, err := os.ReadFile(EnvFilePath())
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

// getEnvOrFile returns the value of an environment variable, falling back to
// the parsed .env file map.
func getEnvOrFile(key string, env map[string]string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return env[key]
}

// parseBool returns true for "true", "1", "yes" (case-insensitive).
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	}
	return false
}
