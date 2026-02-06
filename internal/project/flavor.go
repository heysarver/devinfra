package project

import (
	"fmt"
	"os"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

func AddFlavor(name, flavor string) error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	// Check duplicate
	for _, f := range p.Flavors {
		if f == flavor {
			return fmt.Errorf("flavor %q already added to project %q", flavor, name)
		}
	}

	// Check service name collision
	for _, svc := range p.Services {
		if svc.Name == flavor {
			return fmt.Errorf("flavor name %q conflicts with a service name in project %q", flavor, name)
		}
	}

	// Render flavor template
	data := templateData{
		ProjectName:      name,
		PostgresPassword: randomPassword(24),
		RabbitmqPassword: randomPassword(24),
		MinioPassword:    randomPassword(24),
	}

	ui.Info("Adding flavor '%s' to project '%s'...", flavor, name)
	if err := renderFlavor(p.Dir, flavor, data); err != nil {
		return err
	}

	// Update registry
	p.Flavors = append(p.Flavors, flavor)
	if err := config.SaveRegistry(reg); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	ui.Ok("Flavor '%s' added to '%s'.", flavor, name)
	fmt.Fprintf(os.Stderr, "  File: %s/docker-compose.%s.yaml\n", p.Dir, flavor)
	fmt.Fprintf(os.Stderr, "  Run 'di up %s' to apply.\n", name)
	return nil
}
