package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeFileNames lists Docker Compose filenames in priority order.
var ComposeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

// DetectedService represents a service found in a compose file.
type DetectedService struct {
	Name string
	Port int // 0 means no port exposed
}

// composeFile is a minimal struct for parsing compose YAML.
type composeFile struct {
	Services map[string]serviceDef `yaml:"services"`
}

// serviceDef captures only what we need from a service definition.
type serviceDef struct {
	Ports []yaml.Node `yaml:"ports"`
}

// FindComposeFile scans a directory for a Docker Compose file
// and returns its filename (not full path). Returns empty string if none found.
func FindComposeFile(dir string) string {
	for _, name := range ComposeFileNames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return name
		}
	}
	return ""
}

// ParseServices reads a compose file and extracts service names and ports.
func ParseServices(dir, composeFileName string) ([]DetectedService, error) {
	path := filepath.Join(dir, composeFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading compose file: %w", err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing compose file: %w", err)
	}

	var services []DetectedService
	for name, svc := range cf.Services {
		port := 0
		if len(svc.Ports) > 0 {
			port = extractFirstPort(svc.Ports[0])
		}
		services = append(services, DetectedService{Name: name, Port: port})
	}

	return services, nil
}

// extractFirstPort extracts the container port from a compose port definition.
// Supports short form ("8000:8000", "8000", "127.0.0.1:8000:8000")
// and long form (mapping with target key).
func extractFirstPort(node yaml.Node) int {
	switch node.Kind {
	case yaml.ScalarNode:
		return parseShortPort(node.Value)
	case yaml.MappingNode:
		return parseLongPort(node)
	}
	return 0
}

// parseShortPort handles short-form port syntax:
//   - "8000"          → 8000
//   - "8000:8000"     → 8000
//   - "8080:8000"     → 8000 (container port)
//   - "127.0.0.1:8080:8000" → 8000
func parseShortPort(s string) int {
	// Remove any protocol suffix (e.g., "/tcp", "/udp")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}

	parts := strings.Split(s, ":")
	// The last part with a port is the container port
	// "host:container" or "ip:host:container" or just "container"
	portStr := parts[len(parts)-1]

	// Handle port ranges (e.g., "8000-8005")
	if idx := strings.Index(portStr, "-"); idx >= 0 {
		portStr = portStr[:idx]
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

// parseLongPort handles long-form port syntax:
//
//	target: 8000
//	published: "8080"
func parseLongPort(node yaml.Node) int {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Value == "target" {
			port, err := strconv.Atoi(val.Value)
			if err != nil {
				return 0
			}
			return port
		}
	}
	return 0
}
