package cmd

import (
	"fmt"
	"strings"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

type statusOutput struct {
	Projects []projectStatus `json:"projects"`
}

type projectStatus struct {
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Status   string   `json:"status"`
	URLs     []string `json:"urls"`
	Services []string `json:"services,omitempty"`
	Flavors  []string `json:"flavors,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "List all registered projects and their status",
	GroupID: "project",
	RunE:    runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	if len(reg.Projects) == 0 {
		ui.Info("No projects registered yet. Create one with: di new")
		return nil
	}

	// Single docker ps call for all running containers
	running, err := compose.RunningContainers(ctx)
	if err != nil {
		ui.Warn("Could not query container status: %v", err)
	}

	var output statusOutput

	for _, p := range reg.Projects {
		mode := "docker"
		status := "stopped"

		if p.HostMode {
			mode = "host"
			status = "host"
		} else if _, ok := running[p.Name]; ok {
			status = "running"
		}

		// Build URLs
		urls := []string{fmt.Sprintf("https://%s.test", p.Name)}
		for _, svc := range p.Services {
			urls = append(urls, fmt.Sprintf("https://%s.%s.test", svc.Name, p.Name))
		}

		svcNames := make([]string, len(p.Services))
		for i, s := range p.Services {
			svcNames[i] = fmt.Sprintf("%s:%d", s.Name, s.Port)
		}

		output.Projects = append(output.Projects, projectStatus{
			Name:     p.Name,
			Mode:     mode,
			Status:   status,
			URLs:     urls,
			Services: svcNames,
			Flavors:  p.Flavors,
		})
	}

	if flagJSON {
		return ui.PrintJSON(output)
	}

	// Table output
	headers := []string{"NAME", "MODE", "STATUS", "URLS"}
	var rows [][]string
	for _, p := range output.Projects {
		rows = append(rows, []string{p.Name, p.Mode, p.Status, strings.Join(p.URLs, ", ")})
	}
	fmt.Println()
	ui.PrintTable(headers, rows)
	fmt.Println()
	return nil
}
