package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "Manage devinfra configuration",
	GroupID: "util",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a devinfra configuration value.

Supported keys:
  tld                          Local TLD (e.g. claw, test)
  remote.enabled               Enable cross-device remote domain (true/false)
  remote.domain                Remote base domain (e.g. claw.sarvent.cloud)
  remote.dns_provider          DNS provider for ACME challenge (cloudflare)
  remote.acme_email            Email for Let's Encrypt certificate notifications
  remote.cloudflare_zone_token Cloudflare API token with Zone:DNS:Edit permission`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func init() {
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	switch key {
	case "tld":
		return setTLD(cmd, value)
	case "remote.enabled":
		return setRemoteEnabled(value)
	case "remote.domain":
		return setRemoteValue("REMOTE_DOMAIN", value, config.ValidateRemoteDomain)
	case "remote.dns_provider":
		return setRemoteValue("REMOTE_DNS_PROVIDER", value, nil)
	case "remote.acme_email":
		return setRemoteValue("REMOTE_ACME_EMAIL", value, config.ValidateACMEEmail)
	case "remote.cloudflare_zone_token":
		return setRemoteValue("CF_DNS_API_TOKEN", value, nil)
	default:
		return fmt.Errorf("unsupported config key %q; run 'di config set --help' for supported keys", key)
	}
}

// setRemoteEnabled validates prerequisites then writes REMOTE_ENABLED to .env.
func setRemoteEnabled(value string) error {
	if value != "true" && value != "false" {
		return fmt.Errorf("remote.enabled must be 'true' or 'false'")
	}

	if value == "true" {
		// Validate that required fields are already set
		r := config.Remote()
		var missing []string
		if r.Domain == "" {
			missing = append(missing, "remote.domain")
		}
		if r.ACMEEmail == "" {
			missing = append(missing, "remote.acme_email")
		}
		if r.CloudflareToken == "" {
			missing = append(missing, "remote.cloudflare_zone_token")
		}
		if len(missing) > 0 {
			return fmt.Errorf("cannot enable remote access: the following must be set first:\n  %s\n\nRun 'di config set <key> <value>' for each.", strings.Join(missing, "\n  "))
		}
	}

	if err := writeEnvKey("REMOTE_ENABLED", value); err != nil {
		return fmt.Errorf("writing remote.enabled: %w", err)
	}
	ui.Ok("remote.enabled set to %q", value)
	if value == "true" {
		ui.Info("Run 'di regenerate' to apply remote domain configuration.")
	}
	return nil
}

// setRemoteValue writes a single remote config key to .env after optional validation.
func setRemoteValue(envKey, value string, validate func(string) error) error {
	if validate != nil {
		if err := validate(value); err != nil {
			return err
		}
	}
	if err := writeEnvKey(envKey, value); err != nil {
		return fmt.Errorf("writing %s: %w", envKey, err)
	}
	ui.Ok("%s updated", envKey)
	return nil
}

// writeEnvKey reads the existing .env file, updates or appends the given key,
// and writes it back with 0600 permissions.
func writeEnvKey(key, value string) error {
	envPath := config.EnvFilePath()
	data, _ := os.ReadFile(envPath)
	lines := strings.Split(string(data), "\n")
	found := false
	prefix := key + "="
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			found = true
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0600)
}

func setTLD(cmd *cobra.Command, newTLD string) error {
	ctx := cmd.Context()

	if err := config.ValidateTLD(newTLD); err != nil {
		return err
	}

	current := config.TLD()
	if current == newTLD {
		ui.Info("TLD is already set to %q", newTLD)
		return nil
	}

	ui.Info("Changing TLD from %q to %q...", current, newTLD)

	// Update .env — parse-and-replace only the TLD= line, preserving DNS_PORT etc.
	if err := writeTLDToEnv(newTLD); err != nil {
		return fmt.Errorf("writing TLD to .env: %w", err)
	}

	// Re-extract embedded files (dnsmasq.conf, docker-compose.yaml, tls-infra.yaml)
	// with the new TLD so DNS and Traefik infra configs are updated.
	ui.Info("Re-extracting embedded configs with new TLD...")
	if err := compose.ExtractEmbedded(newTLD); err != nil {
		return fmt.Errorf("extracting embedded configs: %w", err)
	}

	// Regenerate all project overlays, certs, and dynamic configs.
	// This also restarts infra and any previously-running projects.
	return project.RegenerateAll(ctx)
}

// writeTLDToEnv reads the existing .env file and replaces only the TLD= line,
// preserving all other values (e.g. DNS_PORT).
func writeTLDToEnv(newTLD string) error {
	return writeEnvKey("TLD", newTLD)
}
