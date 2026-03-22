package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name           string
		configData     string
		prepare        func(t *testing.T, root string)
		wantSubstrings []string
	}{
		{
			name:           "invalid yaml",
			configData:     "{{invalid yaml",
			wantSubstrings: []string{"parse config"},
		},
		{
			name: "unsupported version",
			configData: `version: 99
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
			wantSubstrings: []string{"unsupported version"},
		},
		{
			name: "no services",
			configData: `version: 1
services: []
`,
			wantSubstrings: []string{"no services configured"},
		},
		{
			name: "empty service name",
			configData: `version: 1
services:
  - name: ""
    files:
      dev: env/api/dev.env
`,
			wantSubstrings: []string{"empty name"},
		},
		{
			name: "duplicate service",
			configData: `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
  - name: api
    files:
      stg: env/api/stg.env
`,
			wantSubstrings: []string{"duplicate service"},
		},
		{
			name: "no files",
			configData: `version: 1
services:
  - name: api
    files: {}
`,
			wantSubstrings: []string{"no environments configured"},
		},
		{
			name: "empty file path",
			configData: `version: 1
services:
  - name: api
    files:
      dev: ""
`,
			wantSubstrings: []string{"empty path"},
		},
		{
			name:           "missing file",
			wantSubstrings: []string{"read config"},
		},
		{
			name: "directory schema",
			configData: `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
			prepare: func(t *testing.T, root string) {
				t.Helper()

				if err := os.MkdirAll(filepath.Join(root, "env.schema", "api.yaml"), 0o750); err != nil {
					t.Fatalf("mkdir schema: %v", err)
				}
			},
			wantSubstrings: []string{"is a directory"},
		},
		{
			name: "unknown fields",
			configData: `version: 1
unexpected: true
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
			wantSubstrings: []string{"parse config"},
		},
		{
			name: "duplicate env files",
			configData: `version: 1
services:
  - name: api
    files:
      dev: env/api/shared.env
      stg: env/api/shared.env
`,
			wantSubstrings: []string{"duplicate env file"},
		},
		{
			name: "missing schema reference",
			configData: `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
			wantSubstrings: []string{"schema", "not found"},
		},
		{
			name: "schema outside repository",
			configData: `version: 1
services:
  - name: api
    schema: ../shared/api.yaml
    files:
      dev: env/api/dev.env
`,
			wantSubstrings: []string{"outside repository"},
		},
		{
			name: "env file outside repository",
			configData: `version: 1
services:
  - name: api
    files:
      dev: ../shared/dev.env
`,
			wantSubstrings: []string{"outside repository"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			configPath := filepath.Join(root, "envdesk.yaml")
			if tt.prepare != nil {
				tt.prepare(t, root)
			}
			if tt.configData != "" {
				if err := os.WriteFile(configPath, []byte(tt.configData), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}

			// Act
			_, err := Load(configPath)

			// Assert
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			for _, want := range tt.wantSubstrings {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Load() error = %q, want substring %q", err.Error(), want)
				}
			}
		})
	}
}

func TestService_EnvironmentsReturnsSorted(t *testing.T) {
	// Arrange
	service := Service{
		Name: "api",
		Files: map[string]string{
			"prod": "/path/prod.env",
			"dev":  "/path/dev.env",
			"stg":  "/path/stg.env",
		},
	}

	// Act
	envs := service.Environments()

	// Assert
	if len(envs) != 3 {
		t.Fatalf("len(envs) = %d, want 3", len(envs))
	}
	if envs[0] != "dev" || envs[1] != "prod" || envs[2] != "stg" {
		t.Fatalf("envs = %v, want [dev prod stg]", envs)
	}
}

func TestService_FilePathReturnsError(t *testing.T) {
	// Arrange
	service := Service{
		Name:  "api",
		Files: map[string]string{"dev": "/path/dev.env"},
	}

	// Act
	_, err := service.FilePath("nonexistent")

	// Assert
	if err == nil {
		t.Fatal("FilePath() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("FilePath() error = %q, want not found", err.Error())
	}
}

func TestProject_ServiceReturnsError(t *testing.T) {
	// Arrange
	project := &Project{
		Services: map[string]Service{},
	}

	// Act
	_, err := project.Service("nonexistent")

	// Assert
	if err == nil {
		t.Fatal("Service() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Service() error = %q, want not found", err.Error())
	}
}

func TestLoad_ResolvesSchemaPath(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	schemaPath := filepath.Join(root, "env.schema", "api.yaml")
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o750); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	if err := os.WriteFile(schemaPath, []byte("keys:\n  APP_ENV:\n    type: string\n"), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	data := `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	project, err := Load(configPath)
	// Assert
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	service, ok := project.Services["api"]
	if !ok {
		t.Fatal("project.Services[\"api\"] missing, want service")
	}
	if service.SchemaPath != schemaPath {
		t.Fatalf("service.SchemaPath = %q, want %q", service.SchemaPath, schemaPath)
	}
}
