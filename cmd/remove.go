package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagForce               bool
	flagNoDirectoryPreserve bool
)

var removeCmd = &cobra.Command{
	Use:     "remove [project]",
	Short:   "Remove a project from the registry",
	Long:    "Stop project containers, remove certificates and dynamic configs, and unregister the project. The project directory is preserved.",
	GroupID: "project",
	Args:    cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:    runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&flagForce, "force", false, "skip confirmation prompt")
	removeCmd.Flags().BoolVar(&flagNoDirectoryPreserve, "no-directory-preserve", false, "also delete the project directory")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Project select when no arg provided
	if len(args) == 0 {
		name, err := pickProject("Select project to remove")
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

	// Verify project exists
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	// Confirm unless --force or --yes
	if !flagForce && !flagYes {
		description := buildRemoveDescription(name, p, flagNoDirectoryPreserve)
		var ok bool
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("Remove project '%s'?", name)).
			Description(description).
			Affirmative("Yes, remove").
			Negative("Cancel").
			Value(&ok)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.Info("Cancelled.")
				return nil
			}
			return err
		}
		if !ok {
			ui.Info("Cancelled.")
			return nil
		}
	}

	return project.Remove(ctx, name, flagNoDirectoryPreserve)
}

func buildRemoveDescription(name string, p *config.Project, noDirectoryPreserve bool) string {
	var lines []string
	lines = append(lines,
		fmt.Sprintf("This will remove project '%s' from devinfra:", name),
		"  - Stop project containers (if running)",
		fmt.Sprintf("  - Delete certs for *.%s.%s", name, config.TLD()),
		"  - Delete Traefik dynamic configs",
		"  - Remove from projects.yaml",
	)
	if noDirectoryPreserve {
		lines = append(lines, fmt.Sprintf("  - DELETE project directory (%s)", p.Dir))
	} else {
		lines = append(lines, fmt.Sprintf("\n  NOTE: The project directory (%s) will NOT be deleted.", p.Dir))
	}
	return strings.Join(lines, "\n")
}
