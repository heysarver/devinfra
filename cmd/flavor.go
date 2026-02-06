package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var flavorCmd = &cobra.Command{
	Use:     "flavor",
	Short:   "Manage project flavors",
	GroupID: "project",
}

var flavorAddCmd = &cobra.Command{
	Use:   "add <project> <flavor>",
	Short: "Add a flavor overlay to a project",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// Complete project names
			return projectNameCompletion(cmd, args, toComplete)
		case 1:
			// Complete flavor names
			return discoverFlavors(), cobra.ShellCompDirectiveNoFileComp
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		project.TemplatesFS = embeddedTemplatesFS
		return project.AddFlavor(args[0], args[1])
	},
}

var flavorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available flavors",
	RunE: func(cmd *cobra.Command, args []string) error {
		flavors := discoverFlavors()

		if flagJSON {
			return ui.PrintJSON(flavors)
		}

		if len(flavors) == 0 {
			ui.Info("No flavors available.")
			return nil
		}

		for _, f := range flavors {
			fmt.Println(f)
		}
		return nil
	},
}

func init() {
	flavorCmd.AddCommand(flavorAddCmd)
	flavorCmd.AddCommand(flavorListCmd)
	rootCmd.AddCommand(flavorCmd)
}
