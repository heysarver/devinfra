package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

// RegenerateAll regenerates overlays, certs, and dynamic configs for all
// registered projects. Projects that were running before regeneration are
// restarted after; stopped projects are left stopped.
//
// Partial failures do not abort the run — failed projects are collected and
// reported at the end. The function returns a non-nil error if any project
// failed.
func RegenerateAll(ctx context.Context) error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Snapshot running state before stopping anything
	running, err := compose.RunningContainers(ctx)
	if err != nil {
		ui.Warn("Could not determine running containers: %v", err)
	}

	infraWasRunning := compose.IsInfraRunning(ctx)

	var failures []string

	for i := range reg.Projects {
		p := &reg.Projects[i]
		wasRunning := len(running[p.Name]) > 0

		// Stop if running
		if wasRunning {
			ui.Info("Stopping %s...", p.Name)
			files := p.ComposeFiles()
			if err := compose.ProjectDown(ctx, p.Name, p.Dir, files); err != nil {
				ui.Warn("Failed to stop %s: %v", p.Name, err)
				// Continue — project may already be stopped
			}
		}

		// Rewrite overlay (non-host-mode projects with services only)
		if !p.HostMode && len(p.Services) > 0 {
			ui.Info("Regenerating overlay for %s...", p.Name)
			if err := generateOverlay(p.Name, p.Dir, p.Services); err != nil {
				ui.Warn("Failed to regenerate overlay for %s: %v", p.Name, err)
				failures = append(failures, p.Name)
				continue
			}
		}

		// Remove old certs (handles TLD change — cleans up old-TLD filenames)
		_ = compose.RemoveCerts(p.Name)

		// Generate new certs
		ui.Info("Regenerating certs for %s...", p.Name)
		if err := compose.GenerateCerts(ctx, p.Name); err != nil {
			ui.Warn("Failed to regenerate certs for %s: %v", p.Name, err)
			failures = append(failures, p.Name)
			continue
		}

		// Update domain in registry to match current TLD
		p.Domain = fmt.Sprintf("*.%s.%s", p.Name, config.TLD())
	}

	// Save updated registry (domain fields reflect new TLD)
	if err := config.SaveRegistry(reg); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	// Restart infra if it was running (picks up re-extracted dnsmasq.conf)
	if infraWasRunning {
		ui.Info("Restarting core infrastructure...")
		if err := compose.Down(ctx); err != nil {
			ui.Warn("Failed to stop infrastructure: %v", err)
		}
		if err := compose.Up(ctx); err != nil {
			ui.Warn("Failed to restart infrastructure: %v", err)
		}
	}

	// Restart projects that were running before, skipping failed ones
	failedSet := make(map[string]bool, len(failures))
	for _, name := range failures {
		failedSet[name] = true
	}

	for _, p := range reg.Projects {
		if !failedSet[p.Name] && len(running[p.Name]) > 0 {
			ui.Info("Restarting %s...", p.Name)
			files := p.ComposeFiles()
			if err := compose.ProjectUp(ctx, p.Name, p.Dir, files); err != nil {
				ui.Warn("Failed to restart %s: %v", p.Name, err)
				failures = append(failures, p.Name)
			} else {
				ui.Ok("Restarted %s", p.Name)
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("regeneration failed for: %s", strings.Join(failures, ", "))
	}
	return nil
}
