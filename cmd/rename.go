package cmd

import (
	"fmt"
	"os"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var flagRenameDir string

var renameCmd = &cobra.Command{
	Use:   "rename <project> [new-name]",
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
	Args:    cobra.RangeArgs(1, 2),
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
	oldName := args[0]
	newName := oldName
	if len(args) == 2 {
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
		fmt.Fprintf(os.Stderr, "Renaming project '%s':\n", oldName)
		if nameChanged {
			fmt.Fprintf(os.Stderr, "  - Stop running containers\n")
			fmt.Fprintf(os.Stderr, "  - Delete certs for *.%s.test\n", oldName)
			fmt.Fprintf(os.Stderr, "  - Generate new certs for *.%s.test\n", newName)
			fmt.Fprintf(os.Stderr, "  - Update registry: name '%s' → '%s'\n", oldName, newName)
		}
		if dirChanged {
			fmt.Fprintf(os.Stderr, "  - Update tracked directory to: %s\n", flagRenameDir)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, "Continue? [y/N] ")
		var answer string
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
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
