package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Service struct {
	Name string `yaml:"name" json:"name"`
	Port int    `yaml:"port" json:"port"`
}

type Project struct {
	Name        string    `yaml:"name" json:"name"`
	Dir         string    `yaml:"dir" json:"dir"`
	Domain      string    `yaml:"domain" json:"domain"`
	HostMode    bool      `yaml:"host_mode" json:"host_mode"`
	Services    []Service `yaml:"services" json:"services"`
	Flavors     []string  `yaml:"flavors,omitempty" json:"flavors,omitempty"`
	ComposeFile string    `yaml:"compose_file,omitempty" json:"compose_file,omitempty"`
	Created     string    `yaml:"created_at" json:"created_at"`
}

type Registry struct {
	Projects []Project `yaml:"projects"`
}

// LoadRegistry reads and parses the projects.yaml file.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry() (*Registry, error) {
	path := RegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}
	return &reg, nil
}

// SaveRegistry writes the registry to projects.yaml using atomic write
// (write to temp file, then rename) to prevent corruption on crash.
func SaveRegistry(reg *Registry) error {
	if err := EnsureDirs(); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}
	path := RegistryPath()
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".projects.yaml.tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming registry: %w", err)
	}
	return nil
}

// Get returns a project by name, or nil if not found.
func (r *Registry) Get(name string) *Project {
	for i := range r.Projects {
		if r.Projects[i].Name == name {
			return &r.Projects[i]
		}
	}
	return nil
}

// Add appends a project to the registry.
func (r *Registry) Add(p Project) error {
	if existing := r.Get(p.Name); existing != nil {
		return fmt.Errorf("project %q already exists", p.Name)
	}
	r.Projects = append(r.Projects, p)
	return nil
}

// Remove deletes a project from the registry by name.
func (r *Registry) Remove(name string) error {
	for i, p := range r.Projects {
		if p.Name == name {
			r.Projects = append(r.Projects[:i], r.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %q not found", name)
}

// List returns all project names.
func (r *Registry) List() []string {
	names := make([]string, len(r.Projects))
	for i, p := range r.Projects {
		names[i] = p.Name
	}
	return names
}

// HasPort returns true if any project already uses the given port.
func (r *Registry) HasPort(port int) (string, bool) {
	for _, p := range r.Projects {
		for _, s := range p.Services {
			if s.Port == port {
				return p.Name, true
			}
		}
	}
	return "", false
}

// HasDir returns the project name if any project is registered at the given directory.
func (r *Registry) HasDir(dir string) (string, bool) {
	canon, _ := CanonicalPath(dir)
	for _, p := range r.Projects {
		pCanon, _ := CanonicalPath(p.Dir)
		if canon == pCanon {
			return p.Name, true
		}
	}
	return "", false
}

// CanonicalPath resolves symlinks and returns a clean absolute path.
func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Directory may not exist yet (e.g., clone target)
		return filepath.Clean(abs), nil
	}
	return filepath.Clean(resolved), nil
}
