package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [project]",
	Short: "Stop infrastructure or a project",
	Long:  "Stop core infrastructure (Traefik, DNSMasq, socket-proxy) or a specific project's containers.",
	GroupID: "infra",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolVar(&flagAll, "all", false, "stop all registered projects")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if flagAll {
		reg, err := config.LoadRegistry()
		if err != nil {
			return err
		}

		for _, p := range reg.Projects {
			if p.HostMode {
				continue
			}
			ui.Info("Stopping %s...", p.Name)
			files := composeFilesForProject(p)
			if err := compose.ProjectDown(ctx, p.Dir, files); err != nil {
				ui.Warn("Failed to stop %s: %v", p.Name, err)
			} else {
				ui.Ok("Stopped %s", p.Name)
			}
		}
		return nil
	}

	if len(args) == 0 {
		ui.Info("Stopping core infrastructure...")
		if err := compose.Down(ctx); err != nil {
			return fmt.Errorf("stopping infrastructure: %w", err)
		}
		ui.Ok("Core infrastructure stopped.")
		return nil
	}

	name := args[0]
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	ui.Info("Stopping %s...", name)
	files := composeFilesForProject(*p)
	if err := compose.ProjectDown(ctx, p.Dir, files); err != nil {
		return fmt.Errorf("stopping %s: %w", name, err)
	}
	ui.Ok("Stopped %s", name)
	return nil
}
