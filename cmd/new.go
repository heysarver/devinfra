package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagNewName     string
	flagNewDir      string
	flagNewMode     string
	flagNewServices string
	flagNewFlavors  string
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new project",
	Long: `Create a new project with Traefik routing, certificates, and optional flavor overlays.

Interactive mode (wizard):
  di new

Non-interactive mode:
  di new --name myapp --dir ~/projects/myapp --services web:3000,api:8080 --flavors postgres`,
	GroupID: "project",
	RunE:   runNew,
}

func init() {
	newCmd.Flags().StringVar(&flagNewName, "name", "", "project name")
	newCmd.Flags().StringVar(&flagNewDir, "dir", "", "project directory")
	newCmd.Flags().StringVar(&flagNewMode, "mode", "docker", "mode: docker or host")
	newCmd.Flags().StringVar(&flagNewServices, "services", "", "services as name:port pairs (e.g., web:3000,api:8080)")
	newCmd.Flags().StringVar(&flagNewFlavors, "flavors", "", "comma-separated flavors (e.g., postgres,redis)")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Set TemplatesFS for project package
	project.TemplatesFS = embeddedTemplatesFS

	// Determine interactive vs non-interactive
	if flagNewName == "" && flagNewDir == "" {
		// Interactive wizard mode
		flavors := discoverFlavors()
		result, err := ui.RunWizard(flavors)
		if err != nil {
			if err.Error() == "cancelled" {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				return nil
			}
			return err
		}

		return project.Create(ctx, project.CreateOpts{
			Name:     result.Name,
			Dir:      result.Dir,
			HostMode: result.HostMode,
			Services: result.Services,
			Flavors:  result.Flavors,
		})
	}

	// Non-interactive mode: both name and dir required
	if flagNewName == "" || flagNewDir == "" {
		return fmt.Errorf("both --name and --dir are required for non-interactive mode")
	}

	// Validate name
	if err := config.ValidateName(flagNewName); err != nil {
		return err
	}

	// Check uniqueness
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	if reg.Get(flagNewName) != nil {
		return fmt.Errorf("project %q already exists", flagNewName)
	}

	// Resolve directory
	dir := expandDir(flagNewDir)

	// Validate directory path
	if err := validateDir(dir); err != nil {
		return err
	}

	// Parse mode
	hostMode := false
	switch strings.ToLower(flagNewMode) {
	case "docker", "":
		hostMode = false
	case "host":
		hostMode = true
	default:
		return fmt.Errorf("invalid mode %q: use 'docker' or 'host'", flagNewMode)
	}

	// Parse services
	services, err := parseServices(flagNewServices)
	if err != nil {
		return err
	}

	// Parse flavors
	var flavors []string
	if flagNewFlavors != "" {
		for _, f := range strings.Split(flagNewFlavors, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				// Verify flavor exists
				available := discoverFlavors()
				found := false
				for _, a := range available {
					if a == f {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("unknown flavor %q; available: %s", f, strings.Join(available, ", "))
				}
				flavors = append(flavors, f)
			}
		}
	}

	return project.Create(ctx, project.CreateOpts{
		Name:     flagNewName,
		Dir:      dir,
		HostMode: hostMode,
		Services: services,
		Flavors:  flavors,
	})
}

func parseServices(s string) ([]config.Service, error) {
	if s == "" {
		// Default: single web service on port 3000
		return []config.Service{{Name: "web", Port: 3000}}, nil
	}

	var services []config.Service
	seenNames := make(map[string]bool)
	seenPorts := make(map[int]bool)

	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid service format: %q (expected name:port)", pair)
		}

		name := parts[0]
		if err := config.ValidateName(name); err != nil {
			return nil, fmt.Errorf("invalid service name %q: %w", name, err)
		}
		if seenNames[name] {
			return nil, fmt.Errorf("duplicate service name: %s", name)
		}

		port, err := config.ParsePort(parts[1])
		if err != nil {
			return nil, err
		}
		if seenPorts[port] {
			return nil, fmt.Errorf("duplicate port: %d", port)
		}

		seenNames[name] = true
		seenPorts[port] = true
		services = append(services, config.Service{Name: name, Port: port})
	}

	return services, nil
}

func expandDir(dir string) string {
	if strings.HasPrefix(dir, "~") {
		dir = filepath.Join(os.Getenv("HOME"), dir[1:])
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func validateDir(dir string) error {
	home := os.Getenv("HOME")
	if home != "" && !strings.HasPrefix(dir, home) {
		ui.Warn("Directory %s is outside $HOME", dir)
	}

	// Reject sensitive directories
	sensitive := []string{"/etc", "/usr", "/var", "/tmp", "/bin", "/sbin"}
	for _, s := range sensitive {
		if strings.HasPrefix(dir, s) {
			return fmt.Errorf("refusing to create project in system directory: %s", dir)
		}
	}

	// Check if directory exists and is non-empty
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(dir)
		if len(entries) > 0 {
			return fmt.Errorf("directory %q already exists and is not empty", dir)
		}
	}

	return nil
}
