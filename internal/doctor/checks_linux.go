//go:build linux

package doctor

import (
	"context"
	"os/exec"
	"strings"
)

func platformChecks(ctx context.Context) []CheckResult {
	var checks []CheckResult

	checks = append(checks, check(ctx, "libnss3-tools", func() bool {
		cmd := exec.CommandContext(ctx, "dpkg", "-s", "libnss3-tools")
		return cmd.Run() == nil
	}, "Install libnss3-tools: sudo apt install libnss3-tools"))

	checks = append(checks, check(ctx, "systemd-resolved .test", func() bool {
		cmd := exec.CommandContext(ctx, "resolvectl", "status")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "test")
	}, "Run 'di init' to configure systemd-resolved for .test domains"))

	checks = append(checks, check(ctx, "Docker group", func() bool {
		cmd := exec.CommandContext(ctx, "id", "-nG")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		for _, g := range strings.Fields(string(out)) {
			if g == "docker" {
				return true
			}
		}
		return false
	}, "Add yourself to the docker group: sudo usermod -aG docker $USER && newgrp docker"))

	return checks
}
