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

	// Detect the project preset from the base compose file
	preset := detectPreset(filepath.Join(p.Dir, "docker-compose.yaml"))

	if len(p.Flavors) == 0 && preset == "" {
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

	// Re-render preset compose if detected
	if preset != "" {
		composePath := filepath.Join(p.Dir, "docker-compose.yaml")
		data := templateData{
			ProjectName:      name,
			PostgresPassword: randomPassword(24),
			RabbitmqPassword: randomPassword(24),
			MinioPassword:    randomPassword(24),
			MysqlPassword:    randomPassword(24),
		}
		if existing, err := extractPasswords(composePath); err == nil {
			mergePasswords(&data, existing)
		}

		ui.Info("Updating %s compose...", preset)
		if err := renderPresetCompose(p.Dir, preset, data); err != nil {
			return fmt.Errorf("updating preset compose: %w", err)
		}

		// Ensure scaffolding directories exist for the preset
		ensurePresetScaffolding(preset, p.Dir)
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

		// Ensure scaffolding directories exist for entrypoint flavors
		ensurePresetScaffolding(flavor, p.Dir)
	}

	ui.Ok("Project '%s' updated.", name)
	fmt.Fprintf(os.Stderr, "  Run 'di up %s' to apply changes.\n", name)
	return nil
}

// detectPreset inspects a compose file and returns the preset name if recognized.
func detectPreset(composePath string) string {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return ""
	}
	var cf struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return ""
	}
	if svc, ok := cf.Services["ghost"]; ok && len(svc.Image) >= 5 && svc.Image[:5] == "ghost" {
		return "ghost"
	}
	if svc, ok := cf.Services["wordpress"]; ok && len(svc.Image) >= 9 && svc.Image[:9] == "wordpress" {
		return "wordpress"
	}
	return ""
}

// ensurePresetScaffolding creates scaffold directories for a preset or entrypoint flavor.
func ensurePresetScaffolding(preset, dir string) {
	var scaffolds []string
	switch preset {
	case "ghost":
		scaffolds = []string{"content/themes", "content/images"}
	case "wordpress":
		scaffolds = []string{"wp-content/themes", "wp-content/plugins"}
	default:
		return
	}
	for _, scaffold := range scaffolds {
		scaffoldDir := filepath.Join(dir, scaffold)
		if err := os.MkdirAll(scaffoldDir, 0755); err != nil {
			ui.Warn("Failed to create %s: %v", scaffold, err)
			continue
		}
		gitkeep := filepath.Join(scaffoldDir, ".gitkeep")
		if _, err := os.Stat(gitkeep); err != nil {
			_ = os.WriteFile(gitkeep, nil, 0644)
		}
		ui.Info("Ensured scaffold directory: %s", scaffold)
	}
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
