package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/project"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagAddName string
	flagAddDir  string
)

var addCmd = &cobra.Command{
	Use:   "add <git-url-or-path>",
	Short: "Import an existing project",
	Long: `Import an existing project into devinfra management.

The argument is auto-detected as either a git URL or local directory path.

Git URL (clones the repo first, then registers):
  di add https://github.com/user/myapp.git
  di add git@github.com:user/myapp.git

Local path (registers an already-cloned directory):
  di add ./existing-project
  di add ~/projects/myapp

Non-interactive mode:
  di add ./existing-project --name myapp --yes`,
	GroupID: "project",
	Args:    cobra.ExactArgs(1),
	RunE:    runAdd,
}

func init() {
	addCmd.Flags().StringVar(&flagAddName, "name", "", "project name (default: derived from directory name)")
	addCmd.Flags().StringVar(&flagAddDir, "dir", "", "clone destination directory (git URL only)")
	rootCmd.AddCommand(addCmd)
}

type inputType int

const (
	inputGitURL inputType = iota
	inputLocalPath
)

// classifyInput determines if the argument is a git URL or local path.
func classifyInput(arg string) inputType {
	if strings.HasPrefix(arg, "git@") ||
		strings.HasPrefix(arg, "git://") ||
		strings.HasPrefix(arg, "https://") ||
		strings.HasPrefix(arg, "http://") ||
		strings.HasPrefix(arg, "ssh://") {
		return inputGitURL
	}
	return inputLocalPath
}

// validateGitURL checks that a git URL uses an allowed scheme.
func validateGitURL(url string) error {
	// Reject dangerous schemes
	if strings.HasPrefix(url, "ext::") {
		return fmt.Errorf("git ext:: transport is not allowed (security risk)")
	}
	if strings.HasPrefix(url, "file://") {
		return fmt.Errorf("file:// URLs are not supported; use a local path instead")
	}
	// Reject flag injection in ssh URLs
	if strings.HasPrefix(url, "ssh://-") {
		return fmt.Errorf("invalid SSH URL")
	}
	return nil
}

// repoNameFromURL extracts a project name from a git URL.
// Example: "https://github.com/user/myapp.git" → "myapp"
func repoNameFromURL(url string) string {
	// Handle SSH URLs: git@github.com:user/myapp.git
	if strings.HasPrefix(url, "git@") {
		if idx := strings.LastIndex(url, "/"); idx >= 0 {
			url = url[idx+1:]
		} else if idx := strings.LastIndex(url, ":"); idx >= 0 {
			url = url[idx+1:]
		}
	} else {
		// HTTPS/SSH scheme URLs
		url = filepath.Base(url)
	}
	url = strings.TrimSuffix(url, ".git")
	return strings.ToLower(url)
}

func runAdd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	arg := args[0]
	kind := classifyInput(arg)

	// Load registry once for all validation
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	var dir string
	var cloned bool
	var derivedName string

	switch kind {
	case inputGitURL:
		if err := validateGitURL(arg); err != nil {
			return err
		}

		derivedName = repoNameFromURL(arg)

		// Determine clone destination
		if flagAddDir != "" {
			dir = expandDir(flagAddDir)
		} else {
			home, err := os.UserHomeDir()
			if err != nil || home == "" {
				return fmt.Errorf("cannot determine home directory; use --dir to specify clone destination")
			}
			defaultDir := filepath.Join(home, "projects", derivedName)
			if flagYes {
				dir = defaultDir
			} else {
				dir = defaultDir
				dirInput := huh.NewInput().
					Title("Clone to directory").
					Placeholder(defaultDir).
					Value(&dir).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("directory is required")
						}
						return nil
					})
				if err := huh.NewForm(huh.NewGroup(dirInput)).Run(); err != nil {
					return err
				}
				dir = expandDir(dir)
			}
		}

		// Check if destination already exists
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(dir)
			if len(entries) > 0 {
				return fmt.Errorf("directory %q already exists and is not empty", dir)
			}
		}

		// Clone
		ui.Info("Cloning %s → %s", arg, dir)
		if err := gitClone(ctx, arg, dir); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		ui.Ok("Cloned successfully")
		cloned = true

	case inputLocalPath:
		dir = expandDir(arg)
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("path %q does not exist or is not a directory", arg)
		}
		derivedName = strings.ToLower(filepath.Base(dir))
	}

	// Check for duplicate directory
	if existingName, found := reg.HasDir(dir); found {
		return fmt.Errorf("directory already registered as project %q; run 'di remove %s' first", existingName, existingName)
	}

	// Determine project name
	name := flagAddName
	if name == "" {
		name = derivedName
	}

	// Prompt for name if invalid or in interactive mode
	if err := config.ValidateName(name); err != nil {
		if flagYes {
			return fmt.Errorf("derived name %q is invalid: %w; use --name to specify", name, err)
		}
		ui.Warn("Derived name %q is not valid: %v", name, err)
		nameInput := huh.NewInput().
			Title("Project name").
			Placeholder(derivedName).
			Value(&name).
			Validate(func(s string) error {
				if err := config.ValidateName(s); err != nil {
					return err
				}
				if reg.Get(s) != nil {
					return fmt.Errorf("project %q already exists", s)
				}
				return nil
			})
		if err := huh.NewForm(huh.NewGroup(nameInput)).Run(); err != nil {
			return err
		}
	} else if !flagYes && flagAddName == "" {
		// Interactive: confirm/change name
		nameInput := huh.NewInput().
			Title("Project name").
			Value(&name).
			Validate(func(s string) error {
				if err := config.ValidateName(s); err != nil {
					return err
				}
				if reg.Get(s) != nil {
					return fmt.Errorf("project %q already exists", s)
				}
				return nil
			})
		if err := huh.NewForm(huh.NewGroup(nameInput)).Run(); err != nil {
			return err
		}
	}

	// Check name uniqueness
	if reg.Get(name) != nil {
		return fmt.Errorf("project %q already exists; run 'di remove %s' first", name, name)
	}

	// Scan for compose file
	composeFileName := compose.FindComposeFile(dir)

	var selectedServices []config.Service

	if composeFileName != "" {
		ui.Info("Found %s", composeFileName)

		detected, err := compose.ParseServices(dir, composeFileName)
		if err != nil {
			ui.Warn("Could not parse compose file: %v", err)
		} else {
			selectedServices, err = promptServiceSelection(detected)
			if err != nil {
				return err
			}
		}
	} else {
		ui.Info("No docker-compose file found")
		selectedServices, err = promptNoCompose()
		if err != nil {
			return err
		}
	}

	// Validate all service names
	for _, svc := range selectedServices {
		if err := config.ValidateName(svc.Name); err != nil {
			return fmt.Errorf("service name %q is invalid: %w", svc.Name, err)
		}
	}

	return project.Add(ctx, project.AddOpts{
		Name:        name,
		Dir:         dir,
		Services:    selectedServices,
		ComposeFile: composeFileName,
		Cloned:      cloned,
	})
}

// promptServiceSelection shows detected services and lets the user pick which get routing.
func promptServiceSelection(detected []compose.DetectedService) ([]config.Service, error) {
	// Sort services: those with ports first, then alphabetically
	sort.Slice(detected, func(i, j int) bool {
		if (detected[i].Port > 0) != (detected[j].Port > 0) {
			return detected[i].Port > 0
		}
		return detected[i].Name < detected[j].Name
	})

	// Show services without ports for transparency
	var withoutPorts []string
	for _, svc := range detected {
		if svc.Port == 0 {
			withoutPorts = append(withoutPorts, svc.Name)
		}
	}
	if len(withoutPorts) > 0 {
		ui.Info("Services without ports (no routing): %s", strings.Join(withoutPorts, ", "))
	}

	// Build options from services with ports
	var portServices []compose.DetectedService
	for _, svc := range detected {
		if svc.Port > 0 {
			portServices = append(portServices, svc)
		}
	}

	if len(portServices) == 0 {
		ui.Info("No services with ports found")
		return nil, nil
	}

	if flagYes {
		// Auto-select all services with ports
		var services []config.Service
		for _, svc := range portServices {
			services = append(services, config.Service{Name: svc.Name, Port: svc.Port})
		}
		return services, nil
	}

	// Interactive multi-select
	opts := make([]huh.Option[string], len(portServices))
	for i, svc := range portServices {
		label := fmt.Sprintf("%s (port %d)", svc.Name, svc.Port)
		opts[i] = huh.NewOption(label, svc.Name).Selected(true)
	}

	var selectedNames []string
	multiSelect := huh.NewMultiSelect[string]().
		Title("Select services for .test domain routing").
		Options(opts...).
		Value(&selectedNames)

	if err := huh.NewForm(huh.NewGroup(multiSelect)).Run(); err != nil {
		return nil, err
	}

	// Build selected services
	var services []config.Service
	for _, name := range selectedNames {
		for _, svc := range portServices {
			if svc.Name == name {
				services = append(services, config.Service{Name: svc.Name, Port: svc.Port})
				break
			}
		}
	}

	return services, nil
}

// promptNoCompose handles the case where no compose file is found.
func promptNoCompose() ([]config.Service, error) {
	if flagYes {
		// Non-interactive: register only, no routing
		return nil, nil
	}

	choice := "register"
	choiceSelect := huh.NewSelect[string]().
		Title("How should this project be configured?").
		Options(
			huh.NewOption("Generate a docker-compose.yaml (like 'di new')", "generate"),
			huh.NewOption("Register only (no routing)", "register"),
		).
		Value(&choice)

	if err := huh.NewForm(huh.NewGroup(choiceSelect)).Run(); err != nil {
		return nil, err
	}

	if choice == "register" {
		return nil, nil
	}

	// Generate: use the existing service prompt from the wizard
	services, err := promptNewServices()
	if err != nil {
		return nil, err
	}

	return services, nil
}

// promptNewServices prompts for service name/port pairs.
func promptNewServices() ([]config.Service, error) {
	var services []config.Service

	for i := 0; ; i++ {
		defaultName := ""
		if i == 0 {
			defaultName = "web"
		}
		defaultPort := "3000"

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
				_, err := config.ParsePort(s)
				return err
			})

		if err := huh.NewForm(huh.NewGroup(nameInput, portInput)).Run(); err != nil {
			return nil, err
		}

		if svcName == "" {
			svcName = defaultName
		}
		if svcPort == "" {
			svcPort = defaultPort
		}

		port, _ := config.ParsePort(svcPort)
		services = append(services, config.Service{Name: svcName, Port: port})

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

// gitClone clones a git repository with security hardening.
func gitClone(ctx context.Context, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--", url, dest)
	cmd.Stderr = os.Stderr // show clone progress
	cmd.Stdout = os.Stderr
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("clone timed out after 5 minutes")
		}
		return err
	}
	return nil
}
