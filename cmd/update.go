package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)


var flagUpdateRestart bool

var updateCmd = &cobra.Command{
	Use:   "update [project]",
	Short: "Re-render flavor overlays from current templates",
	Long:  "Regenerate flavor overlay files for a project using the latest devinfra templates, preserving existing passwords.",
	GroupID: "project",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&flagUpdateRestart, "restart", false, "restart project containers after update")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if len(args) == 0 {
		name, err := pickProject("Select project to update")
		if err != nil {
			return err
		}
		if name == "" {
			ui.Info("Cancelled.")
			return nil
		}
		args = []string{name}
	}
	name := args[0]

	project.TemplatesFS = embeddedTemplatesFS

	if err := project.Update(name); err != nil {
		return err
	}

	if flagUpdateRestart {
		reg, err := config.LoadRegistry()
		if err != nil {
			return err
		}
		p := reg.Get(name)
		if p == nil {
			return fmt.Errorf("project %q not found in registry", name)
		}

		files := p.ComposeFiles()
		ui.Info("Restarting %s...", name)
		if err := compose.ProjectDown(ctx, p.Dir, files); err != nil {
			ui.Warn("Failed to stop %s: %v", name, err)
		}
		if err := compose.ProjectUp(ctx, p.Dir, files); err != nil {
			return fmt.Errorf("starting %s: %w", name, err)
		}
		ui.Ok("Restarted %s", name)
	}

	return nil
}
