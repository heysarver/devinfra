package cmd

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:       "list [projects|flavors]",
	Short:     "List projects or available flavors",
	GroupID:   "project",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"projects", "flavors"},
	RunE:      runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "projects":
		return listProjects()
	case "flavors":
		return listFlavors()
	default:
		return fmt.Errorf("unknown resource: %s (use 'projects' or 'flavors')", args[0])
	}
}

func listProjects() error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	if flagJSON {
		return ui.PrintJSON(reg.List())
	}

	if len(reg.Projects) == 0 {
		ui.Info("No projects registered.")
		return nil
	}

	for _, name := range reg.List() {
		fmt.Println(name)
	}
	return nil
}

func listFlavors() error {
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
}

// discoverFlavors finds available flavor templates from the embedded filesystem.
func discoverFlavors() []string {
	var flavors []string

	// Walk the embedded templates directory for flavor files
	root := filepath.Join("embed", "templates", "flavors")
	fs.WalkDir(embeddedTemplatesFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".yaml.tpl") {
			flavor := strings.TrimSuffix(name, ".yaml.tpl")
			flavors = append(flavors, flavor)
		}
		return nil
	})

	return flavors
}
