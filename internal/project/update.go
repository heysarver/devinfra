package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"gopkg.in/yaml.v3"
)

func Update(name string) error {
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	if len(p.Flavors) == 0 {
		ui.Info("No flavors to update for project '%s'.", name)
		return nil
	}

	// If an entrypoint flavor is present, ensure base compose has no conflicting services
	if hasEntrypointFlavor(p.Flavors) {
		composePath := filepath.Join(p.Dir, "docker-compose.yaml")
		if hasDefaultWebService(composePath) {
			ui.Info("Removing default web service (replaced by flavor)...")
			if err := generateNetworkOnlyCompose(name, p.Dir); err != nil {
				return fmt.Errorf("updating base compose: %w", err)
			}
		}
	}

	for _, flavor := range p.Flavors {
		overlayPath := filepath.Join(p.Dir, fmt.Sprintf("docker-compose.%s.yaml", flavor))

		// Extract passwords from existing overlay (if it exists)
		data := templateData{
			ProjectName:      name,
			PostgresPassword: randomPassword(24),
			RabbitmqPassword: randomPassword(24),
			MinioPassword:    randomPassword(24),
			MysqlPassword:    randomPassword(24),
		}
		if existing, err := extractPasswords(overlayPath); err == nil {
			mergePasswords(&data, existing)
		}

		ui.Info("Updating flavor '%s'...", flavor)
		if err := renderFlavor(p.Dir, flavor, data); err != nil {
			return fmt.Errorf("updating flavor %s: %w", flavor, err)
		}
	}

	ui.Ok("Project '%s' updated.", name)
	fmt.Fprintf(os.Stderr, "  Run 'di up %s' to apply changes.\n", name)
	return nil
}

// passwordMapping maps compose environment variable names to templateData field identifiers.
var passwordMapping = map[string]string{
	"POSTGRES_PASSWORD":      "postgres",
	"RABBITMQ_DEFAULT_PASS":  "rabbitmq",
	"MINIO_ROOT_PASSWORD":    "minio",
	"MYSQL_PASSWORD":         "mysql",
	"MYSQL_ROOT_PASSWORD":    "mysql",
	"WORDPRESS_DB_PASSWORD":          "mysql",
	"database__connection__password": "mysql",
}

// extractPasswords reads an existing compose overlay and extracts known password values.
func extractPasswords(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cf struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}

	passwords := make(map[string]string)
	for _, svc := range cf.Services {
		for envVar, value := range svc.Environment {
			if field, ok := passwordMapping[envVar]; ok {
				if value != "" {
					passwords[field] = value
				}
			}
		}
	}
	return passwords, nil
}

// hasDefaultWebService checks if a compose file contains a "web" service using nginx.
func hasDefaultWebService(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cf struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return false
	}
	svc, ok := cf.Services["web"]
	if !ok {
		return false
	}
	return len(svc.Image) >= 5 && svc.Image[:5] == "nginx"
}

// mergePasswords overwrites templateData password fields with extracted values.
func mergePasswords(data *templateData, passwords map[string]string) {
	if v, ok := passwords["postgres"]; ok {
		data.PostgresPassword = v
	}
	if v, ok := passwords["rabbitmq"]; ok {
		data.RabbitmqPassword = v
	}
	if v, ok := passwords["minio"]; ok {
		data.MinioPassword = v
	}
	if v, ok := passwords["mysql"]; ok {
		data.MysqlPassword = v
	}
}
