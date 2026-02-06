package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/heysarver/devinfra/internal/config"
)

type WizardResult struct {
	Name     string
	Dir      string
	HostMode bool
	Services []config.Service
	Flavors  []string
}

// RunWizard launches the interactive project creation wizard.
func RunWizard(availableFlavors []string) (*WizardResult, error) {
	result := &WizardResult{}

	// Step 1: Name
	nameInput := huh.NewInput().
		Title("Project name").
		Placeholder("myapp").
		Value(&result.Name).
		Validate(func(s string) error {
			if err := config.ValidateName(s); err != nil {
				return err
			}
			reg, err := config.LoadRegistry()
			if err != nil {
				return nil // registry might not exist yet
			}
			if reg.Get(s) != nil {
				return fmt.Errorf("project %q already exists", s)
			}
			return nil
		})

	if err := huh.NewForm(huh.NewGroup(nameInput)).Run(); err != nil {
		return nil, err
	}

	// Step 2: Directory
	defaultDir := filepath.Join(os.Getenv("HOME"), "projects", result.Name)
	result.Dir = defaultDir

	dirInput := huh.NewInput().
		Title("Directory").
		Placeholder(defaultDir).
		Value(&result.Dir).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("directory is required")
			}
			expanded := expandPath(s)
			if info, err := os.Stat(expanded); err == nil && info.IsDir() {
				entries, _ := os.ReadDir(expanded)
				if len(entries) > 0 {
					return fmt.Errorf("directory already exists and is not empty")
				}
			}
			return nil
		})

	if err := huh.NewForm(huh.NewGroup(dirInput)).Run(); err != nil {
		return nil, err
	}
	result.Dir = expandPath(result.Dir)

	// Step 3: Mode
	modeStr := "docker"
	modeSelect := huh.NewSelect[string]().
		Title("Mode").
		Options(
			huh.NewOption("docker — services run in Docker containers", "docker"),
			huh.NewOption("host — services run on your host machine", "host"),
		).
		Value(&modeStr)

	if err := huh.NewForm(huh.NewGroup(modeSelect)).Run(); err != nil {
		return nil, err
	}
	result.HostMode = modeStr == "host"

	// Step 4: Services
	services, err := promptServices()
	if err != nil {
		return nil, err
	}
	result.Services = services

	// Step 5: Flavors
	if len(availableFlavors) > 0 {
		// Filter out flavors that collide with service names
		var validFlavors []string
		svcNames := make(map[string]bool)
		for _, svc := range result.Services {
			svcNames[svc.Name] = true
		}
		for _, f := range availableFlavors {
			if !svcNames[f] {
				validFlavors = append(validFlavors, f)
			}
		}

		if len(validFlavors) > 0 {
			var selectedFlavors []string
			opts := make([]huh.Option[string], len(validFlavors))
			for i, f := range validFlavors {
				opts[i] = huh.NewOption(f, f)
			}

			flavorSelect := huh.NewMultiSelect[string]().
				Title("Flavors (additional infrastructure services)").
				Options(opts...).
				Value(&selectedFlavors)

			if err := huh.NewForm(huh.NewGroup(flavorSelect)).Run(); err != nil {
				return nil, err
			}
			result.Flavors = selectedFlavors
		}
	}

	// Step 6: Confirmation
	var confirmed bool
	summary := buildSummary(result)

	confirmInput := huh.NewConfirm().
		Title("Create project?").
		Description(summary).
		Affirmative("Create").
		Negative("Cancel").
		Value(&confirmed)

	if err := huh.NewForm(huh.NewGroup(confirmInput)).Run(); err != nil {
		return nil, err
	}

	if !confirmed {
		return nil, fmt.Errorf("cancelled")
	}

	return result, nil
}

func promptServices() ([]config.Service, error) {
	var services []config.Service
	defaultPorts := []string{"3000", "4000", "5173", "8080", "8000", "9000"}

	for i := 0; ; i++ {
		defaultName := ""
		if i == 0 {
			defaultName = "web"
		}

		defaultPort := "8080"
		if i < len(defaultPorts) {
			defaultPort = defaultPorts[i]
		}

		var svcName string
		var svcPort string

		nameInput := huh.NewInput().
			Title(fmt.Sprintf("Service #%d name", i+1)).
			Placeholder(defaultName).
			Value(&svcName).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("name is required")
				}
				if err := config.ValidateName(s); err != nil {
					return err
				}
				for _, existing := range services {
					if existing.Name == s {
						return fmt.Errorf("duplicate service name: %s", s)
					}
				}
				return nil
			})

		portInput := huh.NewInput().
			Title("Port").
			Placeholder(defaultPort).
			Value(&svcPort).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("port is required")
				}
				port, err := config.ParsePort(s)
				if err != nil {
					return err
				}
				for _, existing := range services {
					if existing.Port == port {
						return fmt.Errorf("duplicate port: %d", port)
					}
				}
				return nil
			})

		if err := huh.NewForm(huh.NewGroup(nameInput, portInput)).Run(); err != nil {
			return nil, err
		}

		// Apply defaults if empty
		if svcName == "" {
			svcName = defaultName
		}
		if svcPort == "" {
			svcPort = defaultPort
		}

		port, _ := config.ParsePort(svcPort)
		services = append(services, config.Service{Name: svcName, Port: port})

		// Ask if they want another service
		var addMore bool
		addMoreInput := huh.NewConfirm().
			Title("Add another service?").
			Affirmative("Yes").
			Negative("No").
			Value(&addMore)

		if err := huh.NewForm(huh.NewGroup(addMoreInput)).Run(); err != nil {
			return nil, err
		}

		if !addMore {
			break
		}
	}

	return services, nil
}

func buildSummary(r *WizardResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Name:      %s\n", r.Name))
	b.WriteString(fmt.Sprintf("Directory: %s\n", r.Dir))
	mode := "docker"
	if r.HostMode {
		mode = "host"
	}
	b.WriteString(fmt.Sprintf("Mode:      %s\n", mode))
	b.WriteString("\nServices:\n")
	for i, svc := range r.Services {
		if i == 0 {
			b.WriteString(fmt.Sprintf("  %s (port %d) -> https://%s.test, https://%s.%s.test\n",
				svc.Name, svc.Port, r.Name, svc.Name, r.Name))
		} else {
			b.WriteString(fmt.Sprintf("  %s (port %d) -> https://%s.%s.test\n",
				svc.Name, svc.Port, svc.Name, r.Name))
		}
	}

	if len(r.Flavors) > 0 {
		b.WriteString(fmt.Sprintf("\nFlavors: %s", strings.Join(r.Flavors, ", ")))
	}

	return b.String()
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home := os.Getenv("HOME")
		p = filepath.Join(home, p[1:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
