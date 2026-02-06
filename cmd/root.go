package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagConfigDir string
	flagVerbose   bool
	flagQuiet     bool
	flagJSON      bool
	flagNoColor   bool
	flagYes       bool
)

var rootCmd = &cobra.Command{
	Use:   "devinfra",
	Short: "Local development infrastructure manager",
	Long:  "devinfra (di) manages Traefik, DNSMasq, and Docker Compose projects for local development with .test domains and HTTPS.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Apply --no-color
		if flagNoColor || os.Getenv("NO_COLOR") != "" {
			color.NoColor = true
		}

		// Apply --config-dir
		if flagConfigDir != "" {
			os.Setenv("DEVINFRA_HOME", flagConfigDir)
		}

		// Skip init check for commands that don't need config
		skip := map[string]bool{
			"init":       true,
			"doctor":     true,
			"help":       true,
			"version":    true,
			"completion": true,
			"list":       true, // list flavors uses embedded FS, list projects handles empty gracefully
		}
		if skip[cmd.Name()] {
			return nil
		}

		if !config.IsInitialized() {
			return fmt.Errorf("devinfra not initialized; run 'di init' first")
		}
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.EnableTraverseRunHooks = true

	rootCmd.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "", "override config directory")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "debug output")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress non-error output")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "skip confirmations")

	// Command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "infra", Title: "Infrastructure:"},
		&cobra.Group{ID: "project", Title: "Project Management:"},
		&cobra.Group{ID: "util", Title: "Utilities:"},
	)
}
