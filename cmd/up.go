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

var flagAll bool

var upCmd = &cobra.Command{
	Use:   "up [project]",
	Short: "Start infrastructure or a project",
	Long:  "Start core infrastructure (Traefik, DNSMasq, socket-proxy) or a specific project's containers.",
	GroupID: "infra",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVar(&flagAll, "all", false, "start all registered projects")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// If --all, start infra then all projects
	if flagAll {
		if !compose.IsInfraRunning(ctx) {
			ui.Info("Starting core infrastructure...")
			if err := compose.Up(ctx); err != nil {
				return fmt.Errorf("starting infrastructure: %w", err)
			}
			ui.Ok("Core infrastructure started.")
		}

		reg, err := config.LoadRegistry()
		if err != nil {
			return err
		}

		for _, p := range reg.Projects {
			if p.HostMode {
				ui.Warn("Skipping host-mode project %s", p.Name)
				continue
			}
			ui.Info("Starting %s...", p.Name)
			files := composeFilesForProject(p)
			if err := compose.ProjectUp(ctx, p.Dir, files); err != nil {
				ui.Warn("Failed to start %s: %v", p.Name, err)
			} else {
				ui.Ok("Started %s", p.Name)
			}
		}
		return nil
	}

	// If no project specified, start infra only
	if len(args) == 0 {
		ui.Info("Starting core infrastructure...")
		if err := compose.Up(ctx); err != nil {
			return fmt.Errorf("starting infrastructure: %w", err)
		}
		ui.Ok("Core infrastructure started.")
		return nil
	}

	// Start specific project
	name := args[0]
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	// Check if infra is running first
	if !compose.IsInfraRunning(ctx) {
		if flagYes {
			ui.Info("Starting core infrastructure...")
			if err := compose.Up(ctx); err != nil {
				return fmt.Errorf("starting infrastructure: %w", err)
			}
			ui.Ok("Core infrastructure started.")
		} else {
			fmt.Fprintln(os.Stderr, "Core infrastructure is not running.")
			fmt.Fprint(os.Stderr, "Start it now? [Y/n] ")
			var answer string
			fmt.Scanln(&answer)
			if answer == "" || answer == "y" || answer == "Y" || answer == "yes" {
				ui.Info("Starting core infrastructure...")
				if err := compose.Up(ctx); err != nil {
					return fmt.Errorf("starting infrastructure: %w", err)
				}
				ui.Ok("Core infrastructure started.")
			} else {
				return fmt.Errorf("core infrastructure must be running first; run 'di up'")
			}
		}
	}

	ui.Info("Starting %s...", name)
	files := composeFilesForProject(*p)
	if err := compose.ProjectUp(ctx, p.Dir, files); err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}
	ui.Ok("Started %s", name)
	return nil
}

func composeFilesForProject(p config.Project) []string {
	files := []string{filepath.Join(p.Dir, "docker-compose.yaml")}
	for _, f := range p.Flavors {
		files = append(files, filepath.Join(p.Dir, fmt.Sprintf("docker-compose.%s.yaml", f)))
	}
	return files
}

func projectNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	reg, err := config.LoadRegistry()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return reg.List(), cobra.ShellCompDirectiveNoFileComp
}
