package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
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
	Use:   "add [project] [flavor]",
	Short: "Add a flavor overlay to a project",
	Args:  cobra.RangeArgs(0, 2),
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
	RunE: runFlavorAdd,
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

func runFlavorAdd(cmd *cobra.Command, args []string) error {
	// Step 1: project select when first arg absent
	if len(args) < 1 {
		name, err := pickProject("Select project to add flavor to")
		if err != nil {
			return err
		}
		if name == "" {
			ui.Info("Cancelled.")
			return nil
		}
		args = []string{name}
	}

	// Step 2: flavor select when second arg absent
	if len(args) < 2 {
		flavors := discoverFlavors()
		if len(flavors) == 0 {
			return fmt.Errorf("no flavors available")
		}
		var flavorName string
		sel := huh.NewSelect[string]().
			Title("Select flavor").
			Options(huh.NewOptions(flavors...)...).
			Value(&flavorName)
		if err := huh.NewForm(huh.NewGroup(sel)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.Info("Cancelled.")
				return nil
			}
			return err
		}
		args = append(args, flavorName)
	}

	project.TemplatesFS = embeddedTemplatesFS
	return project.AddFlavor(args[0], args[1])
}
