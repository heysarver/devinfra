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
	certsDir := config.CertsDir()
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs dir: %w", err)
	}

	ui.Info("Generating certs for %s.test...", name)
	cmd := exec.CommandContext(ctx, "mkcert", fmt.Sprintf("%s.test", name), fmt.Sprintf("*.%s.test", name))
	cmd.Dir = certsDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}

	// Set restrictive permissions on key file
	keyFile := filepath.Join(certsDir, fmt.Sprintf("%s.test+1-key.pem", name))
	if err := os.Chmod(keyFile, 0600); err != nil {
		ui.Warn("Could not set key file permissions: %v", err)
	}

	// Create Traefik TLS config
	return WriteTLSConfig(name)
}

// GenerateInfraCerts generates TLS certificates for the Traefik dashboard.
func GenerateInfraCerts(ctx context.Context) error {
	certsDir := config.CertsDir()
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs dir: %w", err)
	}

	ui.Info("Generating infrastructure certs...")
	cmd := exec.CommandContext(ctx, "mkcert", "traefik.test", "*.traefik.test")
	cmd.Dir = certsDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}

	// Set restrictive permissions on key file
	keyFile := filepath.Join(certsDir, "traefik.test+1-key.pem")
	if err := os.Chmod(keyFile, 0600); err != nil {
		ui.Warn("Could not set key file permissions: %v", err)
	}

	return nil
}

// WriteTLSConfig writes a Traefik TLS dynamic config for a project.
func WriteTLSConfig(name string) error {
	dynamicDir := config.DynamicDir()
	if err := os.MkdirAll(dynamicDir, 0700); err != nil {
		return fmt.Errorf("creating dynamic dir: %w", err)
	}

	content := fmt.Sprintf(`tls:
  certificates:
    - certFile: /certs/%s.test+1.pem
      keyFile: /certs/%s.test+1-key.pem
`, name, name)

	path := filepath.Join(dynamicDir, fmt.Sprintf("tls-%s.yaml", name))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing TLS config: %w", err)
	}

	ui.Ok("TLS config written for %s.test", name)
	return nil
}

// RemoveCerts removes certificates and TLS config for a project.
func RemoveCerts(name string) error {
	certsDir := config.CertsDir()
	dynamicDir := config.DynamicDir()

	// Remove cert files
	patterns := []string{
		filepath.Join(certsDir, fmt.Sprintf("%s.test*.pem", name)),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			os.Remove(m)
		}
	}

	// Remove TLS config
	os.Remove(filepath.Join(dynamicDir, fmt.Sprintf("tls-%s.yaml", name)))

	// Remove host config if present
	os.Remove(filepath.Join(dynamicDir, fmt.Sprintf("host-%s.yaml", name)))

	return nil
}
