package cmd

import (
	"fmt"
	"strings"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

type inspectOutput struct {
	Name     string           `json:"name"`
	Dir      string           `json:"dir"`
	Domain   string           `json:"domain"`
	Mode     string           `json:"mode"`
	Status   string           `json:"status"`
	Services []config.Service `json:"services"`
	Flavors  []string         `json:"flavors,omitempty"`
	URLs     []string         `json:"urls"`
	Created  string           `json:"created_at"`
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <project>",
	Short: "Show full project detail",
	GroupID: "project",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	name := args[0]

	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	// Determine status
	mode := "docker"
	status := "stopped"
	if p.HostMode {
		mode = "host"
		status = "host"
	} else {
		running, _ := compose.RunningContainers(ctx)
		if _, ok := running[name]; ok {
			status = "running"
		}
	}

	// Build URLs
	urls := []string{fmt.Sprintf("https://%s.test", name)}
	for i, svc := range p.Services {
		if i > 0 {
			urls = append(urls, fmt.Sprintf("https://%s.%s.test", svc.Name, name))
		}
	}

	out := inspectOutput{
		Name:     p.Name,
		Dir:      p.Dir,
		Domain:   p.Domain,
		Mode:     mode,
		Status:   status,
		Services: p.Services,
		Flavors:  p.Flavors,
		URLs:     urls,
		Created:  p.Created,
	}

	if flagJSON {
		return ui.PrintJSON(out)
	}

	// Human output
	fmt.Printf("Name:      %s\n", out.Name)
	fmt.Printf("Directory: %s\n", out.Dir)
	fmt.Printf("Domain:    %s\n", out.Domain)
	fmt.Printf("Mode:      %s\n", out.Mode)
	fmt.Printf("Status:    %s\n", out.Status)
	fmt.Printf("Created:   %s\n", out.Created)
	fmt.Println()
	fmt.Println("Services:")
	for _, svc := range out.Services {
		fmt.Printf("  %s:%d\n", svc.Name, svc.Port)
	}
	if len(out.Flavors) > 0 {
		fmt.Printf("\nFlavors:   %s\n", strings.Join(out.Flavors, ", "))
	}
	fmt.Printf("\nURLs:\n")
	for _, u := range out.URLs {
		fmt.Printf("  %s\n", u)
	}

	return nil
}
