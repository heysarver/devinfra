package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

type CheckResult struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"` // "ok" or "fail"
	Remediation *string `json:"remediation"`
}

type Report struct {
	Passed bool          `json:"passed"`
	Checks []CheckResult `json:"checks"`
	Errors int           `json:"errors"`
}

func check(ctx context.Context, name string, fn func() bool, remediation string) CheckResult {
	if fn() {
		return CheckResult{Name: name, Status: "ok"}
	}
	return CheckResult{Name: name, Status: "fail", Remediation: &remediation}
}

func cmdExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func containerRunning(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", name)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func networkExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", name)
	return cmd.Run() == nil
}

// RunAll executes all health checks and returns a report.
func RunAll(ctx context.Context) Report {
	var mu sync.Mutex
	var checks []CheckResult
	var wg sync.WaitGroup

	add := func(c CheckResult) {
		mu.Lock()
		checks = append(checks, c)
		mu.Unlock()
	}

	// Tool checks (parallel)
	toolChecks := []struct {
		name        string
		fn          func() bool
		remediation string
	}{
		{"Docker", func() bool {
			cmd := exec.CommandContext(ctx, "docker", "info")
			return cmd.Run() == nil
		}, "Install Docker: https://docs.docker.com/get-docker/"},
		{"mkcert", func() bool { return cmdExists("mkcert") },
			"Install mkcert: brew install mkcert (macOS) or apt install mkcert (Ubuntu)"},
		{"mkcert CA", func() bool {
			cmd := exec.CommandContext(ctx, "mkcert", "-CAROOT")
			out, err := cmd.Output()
			if err != nil {
				return false
			}
			dir := strings.TrimSpace(string(out))
			_, err = os.Stat(dir)
			return err == nil
		}, "Run 'mkcert -install' to set up the local CA"},
	}

	for _, tc := range toolChecks {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			add(check(ctx, tc.name, tc.fn, tc.remediation))
		}()
	}

	// Network check
	wg.Add(1)
	go func() {
		defer wg.Done()
		add(check(ctx, "Docker network", func() bool {
			return networkExists(ctx, "traefik")
		}, "Run 'di init' or 'docker network create traefik'"))
	}()

	// Container checks
	containers := []string{"traefik", "socket-proxy", "dnsmasq"}
	for _, c := range containers {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			add(check(ctx, fmt.Sprintf("%s container", c), func() bool {
				return containerRunning(ctx, c)
			}, "Run 'di up' to start infrastructure"))
		}()
	}

	// Cert check
	wg.Add(1)
	go func() {
		defer wg.Done()
		add(check(ctx, "Infra certs", func() bool {
			_, err := os.Stat(filepath.Join(config.CertsDir(), "traefik.test+1.pem"))
			return err == nil
		}, "Run 'di init' to generate infrastructure certs"))
	}()

	// DNS check
	wg.Add(1)
	go func() {
		defer wg.Done()
		add(check(ctx, "DNS resolution", func() bool {
			port := config.DNSPort()
			cmd := exec.CommandContext(ctx, "dig", "+short", "test.test", "@127.0.0.1", "-p", port)
			out, err := cmd.Output()
			if err != nil {
				return false
			}
			return strings.Contains(string(out), "127.0.0.1")
		}, "Run 'di init' to configure DNS, then 'di up' to start DNSMasq"))
	}()

	// Platform-specific checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, c := range platformChecks(ctx) {
			add(c)
		}
	}()

	wg.Wait()

	// Per-project checks (sequential since we load registry)
	reg, err := config.LoadRegistry()
	if err == nil && len(reg.Projects) > 0 {
		for _, p := range reg.Projects {
			checks = append(checks, check(ctx, fmt.Sprintf("%s: directory", p.Name), func() bool {
				_, err := os.Stat(p.Dir)
				return err == nil
			}, fmt.Sprintf("Directory '%s' does not exist", p.Dir)))

			name := p.Name
			checks = append(checks, check(ctx, fmt.Sprintf("%s: certs", name), func() bool {
				pattern := filepath.Join(config.CertsDir(), fmt.Sprintf("*%s*", name))
				matches, _ := filepath.Glob(pattern)
				return len(matches) > 0
			}, fmt.Sprintf("Run 'di certs regen %s'", name)))
		}
	}

	// Build report
	errors := 0
	for _, c := range checks {
		if c.Status == "fail" {
			errors++
		}
	}

	return Report{
		Passed: errors == 0,
		Checks: checks,
		Errors: errors,
	}
}

// PrintReport formats and prints the doctor report to stderr/stdout.
func PrintReport(r Report) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Dev-Infra Health Check")
	fmt.Fprintln(os.Stderr, "======================")
	fmt.Fprintln(os.Stderr)

	for _, c := range r.Checks {
		padded := fmt.Sprintf("%-25s", c.Name)
		if c.Status == "ok" {
			fmt.Fprintf(os.Stderr, "  %s  OK\n", padded)
		} else {
			rem := ""
			if c.Remediation != nil {
				rem = " → " + *c.Remediation
			}
			fmt.Fprintf(os.Stderr, "  %s  FAIL%s\n", padded, rem)
		}
	}

	fmt.Fprintln(os.Stderr)
	if r.Passed {
		ui.Ok("All checks passed!")
	} else {
		ui.Fail("%d check(s) failed. See remediation steps above.", r.Errors)
	}
}
