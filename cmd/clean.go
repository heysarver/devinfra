package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove generated certs and dynamic configs",
	Long:  "Stop infrastructure, remove all generated certificates and Traefik dynamic configs. Keeps projects.yaml.",
	GroupID: "util",
	RunE:  runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&flagForce, "force", false, "skip confirmation prompt")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if !flagForce && !flagYes {
		fmt.Fprintln(os.Stderr, "This will:")
		fmt.Fprintln(os.Stderr, "  - Stop core infrastructure")
		fmt.Fprintln(os.Stderr, "  - Remove all certificates")
		fmt.Fprintln(os.Stderr, "  - Remove all Traefik dynamic configs")
		fmt.Fprintln(os.Stderr, "  - Keep projects.yaml")
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, "Continue? [y/N] ")

		var answer string
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			ui.Info("Cancelled.")
			return nil
		}
	}

	// Stop infrastructure
	ui.Info("Stopping infrastructure...")
	_ = compose.Down(ctx)

	// Remove certs
	certsDir := config.CertsDir()
	certs, _ := filepath.Glob(filepath.Join(certsDir, "*.pem"))
	for _, c := range certs {
		_ = os.Remove(c)
	}

	// Remove dynamic configs
	dynamicDir := config.DynamicDir()
	dynamics, _ := filepath.Glob(filepath.Join(dynamicDir, "*.yaml"))
	for _, d := range dynamics {
		_ = os.Remove(d)
	}

	ui.Ok("Cleaned.")
	return nil
}
