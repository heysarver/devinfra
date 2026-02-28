//go:build darwin

package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/heysarver/devinfra/internal/config"
)

func platformChecks(ctx context.Context) []CheckResult {
	var checks []CheckResult
	tld := config.TLD()
	resolverPath := fmt.Sprintf("/etc/resolver/%s", tld)

	checks = append(checks, check(ctx, "Homebrew", func() bool {
		_, err := exec.LookPath("brew")
		return err == nil
	}, "Install Homebrew: https://brew.sh"))

	checks = append(checks, check(ctx, resolverPath, func() bool {
		_, err := os.Stat(resolverPath)
		return err == nil
	}, "Run 'di init' to configure DNS resolver"))

	checks = append(checks, check(ctx, "Resolver content", func() bool {
		data, err := os.ReadFile(resolverPath)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "nameserver 127.0.0.1")
	}, "Run 'di init' to configure DNS resolver"))

	port := config.DNSPort()
	checks = append(checks, check(ctx, "Resolver port", func() bool {
		data, err := os.ReadFile(resolverPath)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "port "+port)
	}, "Run 'di init' to update resolver port (currently using port "+port+")"))

	return checks
}
