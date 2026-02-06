package project

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

// TemplatesFS must be set by the caller (cmd package) since the embed
// directive lives alongside the template files in cmd/embed/.
var TemplatesFS embed.FS

type CreateOpts struct {
	Name     string
	Dir      string
	HostMode bool
	Services []config.Service
	Flavors  []string
}

type templateData struct {
	ProjectName      string
	PostgresPassword string
	RabbitmqPassword string
	MinioPassword    string
}

func Create(ctx context.Context, opts CreateOpts) error {
	// Ensure config directories exist before writing any files
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("creating config directories: %w", err)
	}

	rb := &rollback{}
	defer func() { rb.execute() }()

	dir := opts.Dir

	// Create project directory
	ui.Info("Creating project directory: %s", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	rb.add(func() error { return os.RemoveAll(dir) })

	// Prepare template data
	data := templateData{
		ProjectName:      opts.Name,
		PostgresPassword: randomPassword(24),
		RabbitmqPassword: randomPassword(24),
		MinioPassword:    randomPassword(24),
	}

	// Render base templates (Makefile, .env.example)
	ui.Info("Rendering templates...")
	if err := renderBaseTemplates(dir, data); err != nil {
		return fmt.Errorf("rendering templates: %w", err)
	}

	// Generate docker-compose or host config
	if opts.HostMode {
		ui.Info("Generating host-mode Traefik config...")
		if err := generateHostConfig(opts.Name, dir, opts.Services); err != nil {
			return fmt.Errorf("generating host config: %w", err)
		}
		rb.add(func() error {
			_ = os.Remove(filepath.Join(config.DynamicDir(), fmt.Sprintf("host-%s.yaml", opts.Name)))
			return nil
		})
	} else {
		ui.Info("Generating docker-compose.yaml...")
		if err := generateDockerCompose(opts.Name, dir, opts.Services); err != nil {
			return fmt.Errorf("generating compose: %w", err)
		}
	}

	// Generate README
	generateReadme(opts.Name, dir, opts.Services)

	// Render flavor overlays
	for _, flavor := range opts.Flavors {
		ui.Info("  Adding flavor: %s", flavor)
		if err := renderFlavor(dir, flavor, data); err != nil {
			return fmt.Errorf("rendering flavor %s: %w", flavor, err)
		}
	}

	// Generate certs
	if err := compose.GenerateCerts(ctx, opts.Name); err != nil {
		return fmt.Errorf("generating certs: %w", err)
	}
	rb.add(func() error {
		_ = compose.RemoveCerts(opts.Name)
		return nil
	})

	// Register in projects.yaml
	ui.Info("Registering project...")
	reg, err := config.LoadRegistry()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	project := config.Project{
		Name:     opts.Name,
		Dir:      dir,
		Domain:   fmt.Sprintf("*.%s.test", opts.Name),
		HostMode: opts.HostMode,
		Services: opts.Services,
		Flavors:  opts.Flavors,
		Created:  time.Now().Format("2006-01-02"),
	}

	if err := reg.Add(project); err != nil {
		return err
	}
	if err := config.SaveRegistry(reg); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}
	rb.add(func() error {
		reg, _ := config.LoadRegistry()
		if reg != nil {
			_ = reg.Remove(opts.Name)
			_ = config.SaveRegistry(reg)
		}
		return nil
	})

	// Success — disarm rollback
	rb.disarm()

	ui.Ok("Project '%s' created!", opts.Name)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Directory:  %s\n", dir)
	fmt.Fprintln(os.Stderr, "  URLs:")
	for i, svc := range opts.Services {
		if i == 0 {
			fmt.Fprintf(os.Stderr, "    https://%s.test\n", opts.Name)
		}
		fmt.Fprintf(os.Stderr, "    https://%s.%s.test\n", svc.Name, opts.Name)
	}
	fmt.Fprintf(os.Stderr, "  Dashboard:  https://traefik.test\n")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Next steps:")
	fmt.Fprintf(os.Stderr, "  cd %s\n", dir)
	fmt.Fprintln(os.Stderr, "  make up     # Start project")
	fmt.Fprintln(os.Stderr, "  make logs   # Tail logs")

	return nil
}

func renderBaseTemplates(dir string, data templateData) error {
	baseDir := filepath.Join("embed", "templates", "base")
	entries, err := fs.ReadDir(TemplatesFS, baseDir)
	if err != nil {
		return fmt.Errorf("reading base templates: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tpl") {
			continue
		}
		content, err := fs.ReadFile(TemplatesFS, filepath.Join(baseDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading template %s: %w", entry.Name(), err)
		}

		outName := strings.TrimSuffix(entry.Name(), ".tpl")
		tmpl, err := template.New(outName).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", entry.Name(), err)
		}

		outPath := filepath.Join(dir, outName)
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", outPath, err)
		}
		if err := tmpl.Execute(f, data); err != nil {
			_ = f.Close()
			return fmt.Errorf("executing template %s: %w", entry.Name(), err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing %s: %w", outPath, err)
		}
	}

	return nil
}

func renderFlavor(dir, flavor string, data templateData) error {
	flavorPath := filepath.Join("embed", "templates", "flavors", flavor+".yaml.tpl")
	content, err := fs.ReadFile(TemplatesFS, flavorPath)
	if err != nil {
		return fmt.Errorf("flavor %q not found", flavor)
	}

	tmpl, err := template.New(flavor).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing flavor template: %w", err)
	}

	outPath := filepath.Join(dir, fmt.Sprintf("docker-compose.%s.yaml", flavor))
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return tmpl.Execute(f, data)
}

func generateDockerCompose(name, dir string, services []config.Service) error {
	var b strings.Builder

	b.WriteString("services:\n")
	for i, svc := range services {
		routerName := fmt.Sprintf("%s-%s", name, svc.Name)

		var rule string
		if i == 0 {
			rule = fmt.Sprintf("Host(`%s.test`) || Host(`%s.%s.test`)", name, svc.Name, name)
		} else {
			rule = fmt.Sprintf("Host(`%s.%s.test`)", svc.Name, name)
		}

		b.WriteString(fmt.Sprintf("  %s:\n", svc.Name))
		b.WriteString("    image: nginx:alpine\n")
		b.WriteString("    networks:\n")
		b.WriteString("      - default\n")
		b.WriteString("      - traefik\n")
		b.WriteString("    labels:\n")
		b.WriteString("      - \"traefik.enable=true\"\n")
		b.WriteString(fmt.Sprintf("      - \"traefik.http.routers.%s.rule=%s\"\n", routerName, rule))
		b.WriteString(fmt.Sprintf("      - \"traefik.http.routers.%s.entrypoints=websecure\"\n", routerName))
		b.WriteString(fmt.Sprintf("      - \"traefik.http.routers.%s.tls=true\"\n", routerName))
		b.WriteString(fmt.Sprintf("      - \"traefik.http.services.%s.loadbalancer.server.port=%d\"\n", routerName, svc.Port))
		b.WriteString("      - \"traefik.docker.network=traefik\"\n")
		b.WriteString("\n")
	}

	b.WriteString("networks:\n")
	b.WriteString("  traefik:\n")
	b.WriteString("    external: true\n")
	b.WriteString("  default:\n")
	b.WriteString(fmt.Sprintf("    name: %s\n", name))

	return os.WriteFile(filepath.Join(dir, "docker-compose.yaml"), []byte(b.String()), 0644)
}

func generateHostConfig(name, dir string, services []config.Service) error {
	var b strings.Builder

	// Traefik file-provider config
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	for i, svc := range services {
		routerName := fmt.Sprintf("%s-%s", name, svc.Name)
		var rule string
		if i == 0 {
			rule = fmt.Sprintf("Host(`%s.test`) || Host(`%s.%s.test`)", name, svc.Name, name)
		} else {
			rule = fmt.Sprintf("Host(`%s.%s.test`)", svc.Name, name)
		}
		b.WriteString(fmt.Sprintf("    %s:\n", routerName))
		b.WriteString(fmt.Sprintf("      rule: \"%s\"\n", rule))
		b.WriteString("      entryPoints:\n")
		b.WriteString("        - websecure\n")
		b.WriteString(fmt.Sprintf("      service: %s\n", routerName))
		b.WriteString("      tls: {}\n")
	}

	b.WriteString("\n")
	b.WriteString("  services:\n")
	for _, svc := range services {
		routerName := fmt.Sprintf("%s-%s", name, svc.Name)
		b.WriteString(fmt.Sprintf("    %s:\n", routerName))
		b.WriteString("      loadBalancer:\n")
		b.WriteString("        servers:\n")
		b.WriteString(fmt.Sprintf("          - url: \"http://host.docker.internal:%d\"\n", svc.Port))
	}

	hostConfigPath := filepath.Join(config.DynamicDir(), fmt.Sprintf("host-%s.yaml", name))
	if err := os.WriteFile(hostConfigPath, []byte(b.String()), 0644); err != nil {
		return err
	}

	// Host-mode compose: just the network
	composeContent := fmt.Sprintf("networks:\n  default:\n    name: %s\n", name)
	return os.WriteFile(filepath.Join(dir, "docker-compose.yaml"), []byte(composeContent), 0644)
}

func generateReadme(name, dir string, services []config.Service) {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", name))
	b.WriteString("## Quick Start\n\n")
	b.WriteString("```bash\n")
	b.WriteString("make up      # Start project\n")
	b.WriteString("make down    # Stop project\n")
	b.WriteString("make logs    # Tail logs\n")
	b.WriteString("make ps      # Show containers\n")
	b.WriteString("```\n\n")
	b.WriteString("## URLs\n\n")
	for i, svc := range services {
		if i == 0 {
			b.WriteString(fmt.Sprintf("- https://%s.test\n", name))
		}
		b.WriteString(fmt.Sprintf("- https://%s.%s.test\n", svc.Name, name))
	}
	b.WriteString("\n## Infrastructure\n\n")
	b.WriteString("This project uses [devinfra](https://github.com/heysarver/devinfra) for local development infrastructure.\n\n")
	b.WriteString("```bash\n")
	b.WriteString("di up      # Start infrastructure\n")
	b.WriteString("di doctor  # Verify everything works\n")
	b.WriteString("```\n")

	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte(b.String()), 0644)
}

func randomPassword(length int) string {
	b := make([]byte, length/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
