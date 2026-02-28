package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseServices(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		want     []DetectedService
	}{
		{
			name:     "single service with short-form port",
			filename: "docker-compose.yaml",
			content: `
services:
  web:
    ports:
      - "3000:3000"
`,
			want: []DetectedService{{Name: "web", Port: 3000}},
		},
		{
			name:     "service name with hyphen",
			filename: "docker-compose.yaml",
			content: `
services:
  my-service:
    ports:
      - "8080:8080"
`,
			want: []DetectedService{{Name: "my-service", Port: 8080}},
		},
		{
			name:     "service without port",
			filename: "docker-compose.yaml",
			content: `
services:
  worker:
    image: redis
`,
			want: []DetectedService{{Name: "worker", Port: 0}},
		},
		{
			name:     "short-form port without host binding",
			filename: "docker-compose.yaml",
			content: `
services:
  app:
    ports:
      - "3000"
`,
			want: []DetectedService{{Name: "app", Port: 3000}},
		},
		{
			name:     "long-form port with target key",
			filename: "docker-compose.yaml",
			content: `
services:
  api:
    ports:
      - target: 3000
        published: 3001
`,
			want: []DetectedService{{Name: "api", Port: 3000}},
		},
		{
			name:     "compose.yaml filename is found by FindComposeFile",
			filename: "compose.yaml",
			content: `
services:
  server:
    ports:
      - "8000:8000"
`,
			want: []DetectedService{{Name: "server", Port: 8000}},
		},
		{
			name:     "multiple services sorted alphabetically",
			filename: "docker-compose.yaml",
			content: `
services:
  worker:
    image: redis
  web:
    ports:
      - "3000:3000"
`,
			want: []DetectedService{
				{Name: "web", Port: 3000},
				{Name: "worker", Port: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("writing compose file: %v", err)
			}

			got, err := ParseServices(dir, tt.filename)
			if err != nil {
				t.Fatalf("ParseServices: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d services, want %d: %v", len(got), len(tt.want), got)
			}
			for i, svc := range got {
				if svc.Name != tt.want[i].Name {
					t.Errorf("service[%d].Name = %q, want %q", i, svc.Name, tt.want[i].Name)
				}
				if svc.Port != tt.want[i].Port {
					t.Errorf("service[%d].Port = %d, want %d", i, svc.Port, tt.want[i].Port)
				}
			}
		})
	}
}

func TestFindComposeFile(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		wantFile string
	}{
		{
			name:     "prefers compose.yaml over docker-compose.yaml",
			files:    []string{"compose.yaml", "docker-compose.yaml"},
			wantFile: "compose.yaml",
		},
		{
			name:     "finds docker-compose.yaml when no compose.yaml",
			files:    []string{"docker-compose.yaml"},
			wantFile: "docker-compose.yaml",
		},
		{
			name:     "returns empty string when no compose file present",
			files:    []string{"main.go"},
			wantFile: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("services: {}"), 0644); err != nil {
					t.Fatalf("writing %s: %v", f, err)
				}
			}

			got := FindComposeFile(dir)
			if got != tt.wantFile {
				t.Errorf("FindComposeFile = %q, want %q", got, tt.wantFile)
			}
		})
	}
}
