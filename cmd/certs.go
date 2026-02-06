package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var certsCmd = &cobra.Command{
	Use:     "certs",
	Short:   "Manage TLS certificates",
	GroupID: "util",
}

var certsRegenCmd = &cobra.Command{
	Use:   "regen [project]",
	Short: "Regenerate certificates",
	Long:  "Regenerate infrastructure certificates, or a specific project's certificates.",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if len(args) == 0 {
			// Regenerate all: infra + all projects
			ui.Info("Regenerating infrastructure certs...")
			if err := compose.GenerateInfraCerts(ctx); err != nil {
				return fmt.Errorf("regenerating infra certs: %w", err)
			}

			reg, err := config.LoadRegistry()
			if err != nil {
				return err
			}
			for _, p := range reg.Projects {
				ui.Info("Regenerating certs for %s...", p.Name)
				if err := compose.GenerateCerts(ctx, p.Name); err != nil {
					ui.Warn("Failed to regenerate certs for %s: %v", p.Name, err)
				}
			}

			ui.Ok("All certificates regenerated.")
			return nil
		}

		// Regenerate for specific project
		name := args[0]
		reg, err := config.LoadRegistry()
		if err != nil {
			return err
		}
		if reg.Get(name) == nil {
			return fmt.Errorf("project %q not found in registry", name)
		}

		if err := compose.GenerateCerts(ctx, name); err != nil {
			return fmt.Errorf("regenerating certs for %s: %w", name, err)
		}
		ui.Ok("Certificates regenerated for %s.", name)
		return nil
	},
}

func init() {
	certsCmd.AddCommand(certsRegenCmd)
	rootCmd.AddCommand(certsCmd)
}
