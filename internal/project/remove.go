package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

func Remove(ctx context.Context, name string, removeDir bool) error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	projectDir := p.Dir

	// Stop project containers
	composeFile := filepath.Join(projectDir, "docker-compose.yaml")
	if _, err := os.Stat(composeFile); err == nil {
		ui.Info("Stopping project containers...")
		_ = compose.ProjectDown(ctx, projectDir, []string{composeFile})
	}

	// Remove certs
	ui.Info("Removing certs...")
	_ = compose.RemoveCerts(name)

	// Remove from registry
	ui.Info("Removing from registry...")
	if err := reg.Remove(name); err != nil {
		return err
	}
	if err := config.SaveRegistry(reg); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	// Remove project directory if requested
	if removeDir {
		ui.Info("Removing project directory: %s", projectDir)
		if err := os.RemoveAll(projectDir); err != nil {
			return fmt.Errorf("removing project directory: %w", err)
		}
		ui.Ok("Project '%s' removed from devinfra (directory deleted).", name)
	} else {
		ui.Ok("Project '%s' removed from devinfra.", name)
		fmt.Fprintf(os.Stderr, "  Project directory preserved: %s\n", projectDir)
	}

	return nil
}
