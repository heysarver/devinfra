package compose

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

//go:embed embed/compose/docker-compose.yaml embed/compose/dnsmasq.conf
var embeddedCompose embed.FS

//go:embed embed/dynamic/tls-infra.yaml
var embeddedDynamic embed.FS

//go:embed all:embed/scripts
var embeddedScripts embed.FS

// embedData holds the template data used when rendering embedded files.
type embedData struct {
	TLD           string
	RemoteEnabled bool
	ACMEEmail     string
}

// renderTemplate renders src as a Go template with the given data and returns the result.
func renderTemplate(name string, src []byte, data embedData) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// ExtractEmbedded writes embedded compose files to the config directory,
// rendering each file with the given TLD and current remote config.
func ExtractEmbedded(tld string) error {
	remote := config.Remote()
	data := embedData{
		TLD:           tld,
		RemoteEnabled: remote.Enabled,
		ACMEEmail:     remote.ACMEEmail,
	}

	entries := []struct {
		embedPath string
		destPath  string
	}{
		{"embed/compose/docker-compose.yaml", config.ComposeFile()},
		{"embed/compose/dnsmasq.conf", config.DnsmasqConf()},
		{"embed/dynamic/tls-infra.yaml", filepath.Join(config.DynamicDir(), "tls-infra.yaml")},
	}

	for _, e := range entries {
		src, err := embeddedCompose.ReadFile(e.embedPath)
		if err != nil {
			// Try from dynamic embed
			src, err = embeddedDynamic.ReadFile(e.embedPath)
			if err != nil {
				return fmt.Errorf("reading embedded %s: %w", e.embedPath, err)
			}
		}
		rendered, err := renderTemplate(e.embedPath, src, data)
		if err != nil {
			return err
		}
		if err := os.WriteFile(e.destPath, rendered, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", e.destPath, err)
		}
	}

	return nil
}

// ExtractSetupScript extracts and returns the path to a platform setup script,
// rendering it with the given TLD substituted for {{.TLD}} placeholders.
func ExtractSetupScript(platform, tld string) (string, error) {
	name := fmt.Sprintf("setup-%s.sh", strings.ToLower(platform))
	embedPath := filepath.Join("embed", "scripts", name)
	data, err := fs.ReadFile(embeddedScripts, embedPath)
	if err != nil {
		return "", fmt.Errorf("no setup script for platform %s: %w", platform, err)
	}

	rendered, err := renderTemplate(name, data, embedData{TLD: tld})
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(config.ConfigDir(), "scripts")
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, name)
	if err := os.WriteFile(destPath, rendered, 0755); err != nil {
		return "", err
	}
	return destPath, nil
}

// EnsureAcmeDir creates the ACME certificate storage directory with
// restricted permissions (0700). Called before starting infra when remote is enabled.
func EnsureAcmeDir() error {
	return os.MkdirAll(filepath.Join(config.ConfigDir(), "acme"), 0700)
}

// Up starts the core infrastructure containers.
func Up(ctx context.Context) error {
	if config.RemoteEnabled() {
		if err := EnsureAcmeDir(); err != nil {
			return fmt.Errorf("creating ACME directory: %w", err)
		}
	}
	return run(ctx, config.ComposeDir(), "up", "-d")
}

// Down stops the core infrastructure containers.
func Down(ctx context.Context) error {
	return run(ctx, config.ComposeDir(), "down")
}

// Logs tails logs from core infrastructure.
func Logs(ctx context.Context) error {
	return runAttached(ctx, config.ComposeDir(), "logs", "-f")
}

// IsInfraRunning checks if the core infrastructure containers are running.
func IsInfraRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", "traefik")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// ProjectUp starts a specific project's containers.
func ProjectUp(ctx context.Context, name, dir string, composeFiles []string) error {
	args := buildComposeArgs(name, composeFiles)
	args = append(args, "up", "-d")
	return runRaw(ctx, dir, args...)
}

// ProjectDown stops a specific project's containers.
func ProjectDown(ctx context.Context, name, dir string, composeFiles []string) error {
	args := buildComposeArgs(name, composeFiles)
	args = append(args, "down")
	return runRaw(ctx, dir, args...)
}

// ProjectLogs tails logs from a specific project.
func ProjectLogs(ctx context.Context, name, dir string, composeFiles []string) error {
	args := buildComposeArgs(name, composeFiles)
	args = append(args, "logs", "-f")
	return runRawAttached(ctx, dir, args...)
}

// RunningContainers returns a map of compose project name -> list of running container names.
// Uses a single docker ps call for efficiency.
func RunningContainers(ctx context.Context) (map[string][]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Labels}}\t{{.Names}}", "--filter", "status=running")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}

	result := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		labels := parts[0]
		name := parts[1]
		// Extract com.docker.compose.project from labels
		for _, label := range strings.Split(labels, ",") {
			if strings.HasPrefix(label, "com.docker.compose.project=") {
				project := strings.TrimPrefix(label, "com.docker.compose.project=")
				result[project] = append(result[project], name)
				break
			}
		}
	}
	return result, nil
}

// CreateNetwork creates the traefik Docker network if it doesn't exist.
func CreateNetwork(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "create", "traefik")
	// Ignore error if network already exists
	_ = cmd.Run()
	return nil
}

func buildComposeArgs(name string, files []string) []string {
	args := []string{"compose", "-p", name}
	for _, f := range files {
		args = append(args, "-f", f)
	}
	return args
}

func run(ctx context.Context, dir string, composeArgs ...string) error {
	args := append([]string{"compose", "-p", "devinfra"}, composeArgs...)
	ui.Info("Running: docker %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = infraEnv()
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runAttached(ctx context.Context, dir string, composeArgs ...string) error {
	args := append([]string{"compose", "-p", "devinfra"}, composeArgs...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = infraEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// infraEnv returns the environment for infra (devinfra) compose commands,
// including DNS_PORT and CF_DNS_API_TOKEN when remote is enabled.
func infraEnv() []string {
	env := append(os.Environ(), "DNS_PORT="+config.DNSPort())
	if r := config.Remote(); r.Enabled && r.CloudflareToken != "" {
		env = append(env, "CF_DNS_API_TOKEN="+r.CloudflareToken)
	}
	return env
}

func runRaw(ctx context.Context, dir string, args ...string) error {
	ui.Info("Running: docker %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("DNS_PORT=%s", config.DNSPort()))
	cmd.Stdout = os.Stderr // Progress output goes to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runRawAttached(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("DNS_PORT=%s", config.DNSPort()))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
