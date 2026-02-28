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
	Long:  "Set a devinfra configuration value. Supported keys: tld",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
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
	default:
		return fmt.Errorf("unsupported config key %q; supported keys: tld", key)
	}
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
	envPath := config.EnvFilePath()
	data, _ := os.ReadFile(envPath) // ok if missing — we'll create the file
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "TLD=") {
			lines[i] = "TLD=" + newTLD
			found = true
		}
	}
	if !found {
		lines = append(lines, "TLD="+newTLD)
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}
