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

var flagRenameDir string

var renameCmd = &cobra.Command{
	Use:   "rename [project] [new-name]",
	Short: "Rename a project and/or update its tracked directory",
	Long: `Update a project's name in the registry and/or change the directory it tracks.

When renaming:
  - Running containers are stopped
  - Old certificates are removed and new ones are generated
  - The registry is updated with the new name and domain

When changing the directory (--dir):
  - The tracked path in the registry is updated
  - No certificates or configs are changed
  - Useful after manually moving a project directory

Examples:
  di rename myapp newapp
  di rename myapp --dir /new/path/to/myapp
  di rename myapp newapp --dir /new/path/to/newapp`,
	GroupID: "project",
	Args:    cobra.MaximumNArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return projectNameCompletion(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: runRename,
}

func init() {
	renameCmd.Flags().StringVar(&flagRenameDir, "dir", "", "new directory path to track for this project")
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Project select when no arg provided
	if len(args) == 0 {
		name, err := pickProject("Select project to rename")
		if err != nil {
			return err
		}
		if name == "" {
			ui.Info("Cancelled.")
			return nil
		}
		args = []string{name}
	}
	oldName := args[0]

	// New-name input when both new-name arg and --dir are absent
	newName := oldName
	if len(args) < 2 && flagRenameDir == "" {
		nameInput := huh.NewInput().
			Title("New project name").
			Value(&newName).
			Validate(func(s string) error { return config.ValidateName(s) })
		if err := huh.NewForm(huh.NewGroup(nameInput)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.Info("Cancelled.")
				return nil
			}
			return err
		}
	} else if len(args) == 2 {
		newName = args[1]
	}

	nameChanged := newName != oldName
	dirChanged := flagRenameDir != ""

	if !nameChanged && !dirChanged {
		return fmt.Errorf("nothing to do: provide a new name and/or --dir")
	}

	// Verify project exists
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(oldName)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", oldName)
	}

	// Confirm unless --yes
	if !flagYes {
		description := buildRenameDescription(oldName, newName, nameChanged, dirChanged, flagRenameDir)
		var ok bool
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("Rename project '%s'?", oldName)).
			Description(description).
			Affirmative("Yes, rename").
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

	return project.Rename(ctx, project.RenameOpts{
		OldName: oldName,
		NewName: newName,
		NewDir:  flagRenameDir,
	})
}

func buildRenameDescription(oldName, newName string, nameChanged, dirChanged bool, newDir string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Renaming project '%s':", oldName))
	if nameChanged {
		lines = append(lines,
			"  - Stop running containers",
			fmt.Sprintf("  - Delete certs for *.%s.test", oldName),
			fmt.Sprintf("  - Generate new certs for *.%s.test", newName),
			fmt.Sprintf("  - Update registry: name '%s' → '%s'", oldName, newName),
		)
	}
	if dirChanged {
		lines = append(lines, fmt.Sprintf("  - Update tracked directory to: %s", newDir))
	}
	return strings.Join(lines, "\n")
}
