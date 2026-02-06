package cmd

import (
	"fmt"
	"os"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagForce              bool
	flagNoDirectoryPreserve bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <project>",
	Short: "Remove a project from the registry",
	Long:  "Stop project containers, remove certificates and dynamic configs, and unregister the project. The project directory is preserved.",
	GroupID: "project",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&flagForce, "force", false, "skip confirmation prompt")
	removeCmd.Flags().BoolVar(&flagNoDirectoryPreserve, "no-directory-preserve", false, "also delete the project directory")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
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
		fmt.Fprintf(os.Stderr, "This will remove project '%s' from devinfra:\n", name)
		fmt.Fprintf(os.Stderr, "  - Stop project containers (if running)\n")
		fmt.Fprintf(os.Stderr, "  - Delete certs for *.%s.test\n", name)
		fmt.Fprintf(os.Stderr, "  - Delete Traefik dynamic configs\n")
		fmt.Fprintf(os.Stderr, "  - Remove from projects.yaml\n")
		if flagNoDirectoryPreserve {
			fmt.Fprintf(os.Stderr, "  - DELETE project directory (%s)\n", p.Dir)
		}
		fmt.Fprintln(os.Stderr)
		if !flagNoDirectoryPreserve {
			fmt.Fprintf(os.Stderr, "  NOTE: The project directory (%s) will NOT be deleted.\n", p.Dir)
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

	return project.Remove(ctx, name, flagNoDirectoryPreserve)
}
