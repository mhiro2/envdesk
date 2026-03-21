package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RejectsInvalidYAML(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	if err := os.WriteFile(configPath, []byte("{{invalid yaml"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Load() error = %q, want parse config failure", err.Error())
	}
}

func TestLoad_RejectsUnsupportedVersion(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 99
services:
  - name: api
    files:
      dev: env/api/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unsupported version") {
		t.Fatalf("Load() error = %q, want unsupported version failure", err.Error())
	}
}

func TestLoad_RejectsNoServices(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services: []
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no services configured") {
		t.Fatalf("Load() error = %q, want no services failure", err.Error())
	}
}

func TestLoad_RejectsEmptyServiceName(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: ""
    files:
      dev: env/api/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("Load() error = %q, want empty name failure", err.Error())
	}
}

func TestLoad_RejectsDuplicateService(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
  - name: api
    files:
      stg: env/api/stg.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate service") {
		t.Fatalf("Load() error = %q, want duplicate service failure", err.Error())
	}
}

func TestLoad_RejectsNoFiles(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    files: {}
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no environments configured") {
		t.Fatalf("Load() error = %q, want no files failure", err.Error())
	}
}

func TestLoad_RejectsEmptyFilePath(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    files:
      dev: ""
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("Load() error = %q, want empty path failure", err.Error())
	}
}

func TestLoad_RejectsMissingFile(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "read config") {
		t.Fatalf("Load() error = %q, want read config failure", err.Error())
	}
}

func TestLoad_RejectsDirectorySchema(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	schemaDir := filepath.Join(root, "env.schema")
	if err := os.MkdirAll(filepath.Join(schemaDir, "api.yaml"), 0o750); err != nil {
		t.Fatalf("mkdir schema: %v", err)
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
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("Load() error = %q, want directory schema failure", err.Error())
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

func TestLoad_RejectsUnknownFields(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
unexpected: true
services:
  - name: api
    files:
      dev: env/api/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Load() error = %q, want parse config failure", err.Error())
	}
}

func TestLoad_RejectsDuplicateEnvFiles(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    files:
      dev: env/api/shared.env
      stg: env/api/shared.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate env file") {
		t.Fatalf("Load() error = %q, want duplicate env file failure", err.Error())
	}
}

func TestLoad_RejectsMissingSchemaReference(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
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
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "schema") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Load() error = %q, want missing schema failure", err.Error())
	}
}

func TestLoad_RejectsSchemaOutsideRepository(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    schema: ../shared/api.yaml
    files:
      dev: env/api/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("Load() error = %q, want repository boundary failure", err.Error())
	}
}

func TestLoad_RejectsEnvFileOutsideRepository(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	data := `version: 1
services:
  - name: api
    files:
      dev: ../shared/dev.env
`
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Act
	_, err := Load(configPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("Load() error = %q, want repository boundary failure", err.Error())
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
