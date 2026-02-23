package project

import (
	"context"
	"fmt"
	"os"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

type RenameOpts struct {
	OldName string
	NewName string // if same as OldName, only dir changes
	NewDir  string // if empty, keep existing dir
}

func Rename(ctx context.Context, opts RenameOpts) error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	p := reg.Get(opts.OldName)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", opts.OldName)
	}

	nameChanged := opts.NewName != opts.OldName
	dirChanged := opts.NewDir != "" && opts.NewDir != p.Dir

	// Validate new name
	if nameChanged {
		if err := config.ValidateName(opts.NewName); err != nil {
			return fmt.Errorf("invalid name: %w", err)
		}
		if existing := reg.Get(opts.NewName); existing != nil {
			return fmt.Errorf("project %q already exists", opts.NewName)
		}
	}

	// Validate new dir
	if dirChanged {
		if _, err := os.Stat(opts.NewDir); err != nil {
			return fmt.Errorf("directory %q does not exist: %w", opts.NewDir, err)
		}
		if owner, ok := reg.HasDir(opts.NewDir); ok && owner != opts.OldName {
			return fmt.Errorf("directory %q is already registered to project %q", opts.NewDir, owner)
		}
	}

	// Update name-dependent resources
	if nameChanged {
		// Stop containers before changing certs/configs
		files := p.ComposeFiles()
		if len(files) > 0 {
			ui.Info("Stopping project containers...")
			_ = compose.ProjectDown(ctx, p.Dir, files)
		}

		// Remove old certs, TLS config, and host config (if any)
		ui.Info("Removing old certs for %s.test...", opts.OldName)
		_ = compose.RemoveCerts(opts.OldName)

		// Generate certs for new domain
		if err := compose.GenerateCerts(ctx, opts.NewName); err != nil {
			return fmt.Errorf("generating certs: %w", err)
		}

		// For host-mode: regenerate the Traefik file-provider config with new name
		if p.HostMode {
			dir := p.Dir
			if dirChanged {
				dir = opts.NewDir
			}
			if err := generateHostConfig(opts.NewName, dir, p.Services); err != nil {
				return fmt.Errorf("regenerating host config: %w", err)
			}
		}
	}

	// Update registry entry in-place
	for i := range reg.Projects {
		if reg.Projects[i].Name == opts.OldName {
			if nameChanged {
				reg.Projects[i].Name = opts.NewName
				reg.Projects[i].Domain = fmt.Sprintf("*.%s.test", opts.NewName)
			}
			if dirChanged {
				reg.Projects[i].Dir = opts.NewDir
			}
			break
		}
	}

	if err := config.SaveRegistry(reg); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	displayName := opts.NewName
	if !nameChanged {
		displayName = opts.OldName
	}

	if nameChanged && dirChanged {
		ui.Ok("Project '%s' renamed to '%s' and directory updated to %s.", opts.OldName, opts.NewName, opts.NewDir)
	} else if nameChanged {
		ui.Ok("Project '%s' renamed to '%s'.", opts.OldName, opts.NewName)
	} else {
		ui.Ok("Project '%s' directory updated to %s.", opts.OldName, opts.NewDir)
	}

	if nameChanged && !p.HostMode {
		fmt.Fprintln(os.Stderr)
		ui.Warn("docker-compose files may still reference '%s' in Traefik labels and network names.", opts.OldName)
		fmt.Fprintf(os.Stderr, "  Update Host rules and router names to use '%s.test', then run:\n", opts.NewName)
		fmt.Fprintf(os.Stderr, "  di up %s\n", displayName)
	} else if nameChanged {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  Run 'di up %s' to restart.\n", displayName)
	}

	return nil
}
