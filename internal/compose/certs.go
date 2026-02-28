package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
)

// GenerateCerts generates TLS certificates for a project using mkcert.
func GenerateCerts(ctx context.Context, name string) error {
	tld := config.TLD()
	certsDir := config.CertsDir()
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs dir: %w", err)
	}

	ui.Info("Generating certs for %s.%s...", name, tld)
	cmd := exec.CommandContext(ctx, "mkcert", fmt.Sprintf("%s.%s", name, tld), fmt.Sprintf("*.%s.%s", name, tld))
	cmd.Dir = certsDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}

	// Set restrictive permissions on key file
	keyFile := filepath.Join(certsDir, fmt.Sprintf("%s.%s+1-key.pem", name, tld))
	if err := os.Chmod(keyFile, 0600); err != nil {
		ui.Warn("Could not set key file permissions: %v", err)
	}

	// Create Traefik TLS config
	return WriteTLSConfig(name)
}

// GenerateInfraCerts generates TLS certificates for the Traefik dashboard.
func GenerateInfraCerts(ctx context.Context) error {
	tld := config.TLD()
	certsDir := config.CertsDir()
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs dir: %w", err)
	}

	ui.Info("Generating infrastructure certs...")
	cmd := exec.CommandContext(ctx, "mkcert", fmt.Sprintf("traefik.%s", tld), fmt.Sprintf("*.traefik.%s", tld))
	cmd.Dir = certsDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}

	// Set restrictive permissions on key file
	keyFile := filepath.Join(certsDir, fmt.Sprintf("traefik.%s+1-key.pem", tld))
	if err := os.Chmod(keyFile, 0600); err != nil {
		ui.Warn("Could not set key file permissions: %v", err)
	}

	return nil
}

// WriteTLSConfig writes a Traefik TLS dynamic config for a project.
func WriteTLSConfig(name string) error {
	tld := config.TLD()
	dynamicDir := config.DynamicDir()
	if err := os.MkdirAll(dynamicDir, 0700); err != nil {
		return fmt.Errorf("creating dynamic dir: %w", err)
	}

	content := fmt.Sprintf(`tls:
  certificates:
    - certFile: /certs/%s.%s+1.pem
      keyFile: /certs/%s.%s+1-key.pem
`, name, tld, name, tld)

	path := filepath.Join(dynamicDir, fmt.Sprintf("tls-%s.yaml", name))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing TLS config: %w", err)
	}

	ui.Ok("TLS config written for %s.%s", name, tld)
	return nil
}

// RemoveCerts removes certificates and TLS config for a project.
func RemoveCerts(name string) error {
	tld := config.TLD()
	certsDir := config.CertsDir()
	dynamicDir := config.DynamicDir()

	// Remove cert files matching either the current TLD or any TLD (glob)
	patterns := []string{
		filepath.Join(certsDir, fmt.Sprintf("%s.%s*.pem", name, tld)),
		// Also catch certs from a previous TLD if the user changed it
		filepath.Join(certsDir, fmt.Sprintf("%s.*+*.pem", name)),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	// Remove TLS config
	_ = os.Remove(filepath.Join(dynamicDir, fmt.Sprintf("tls-%s.yaml", name)))

	// Remove host config if present
	_ = os.Remove(filepath.Join(dynamicDir, fmt.Sprintf("host-%s.yaml", name)))

	return nil
}
