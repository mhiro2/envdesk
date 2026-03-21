package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestInit_EmptyRepository(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")

	// Act
	result, err := app.Init(t.Context(), nil, app.InitOptions{
		ConfigPath:   configPath,
		ScaffoldSOPS: true,
	})
	// Assert
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	if len(result.Files) != 6 {
		t.Fatalf("len(result.Files) = %d, want 6", len(result.Files))
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if string(configData) != `version: 1

services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
` {
		t.Fatalf("config file = %q, want scaffolded config", string(configData))
	}

	schemaData, err := os.ReadFile(filepath.Join(root, "env.schema/api.yaml"))
	if err != nil {
		t.Fatalf("read schema file: %v", err)
	}
	if string(schemaData) != `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg, prod]
    secret: false
` {
		t.Fatalf("schema file = %q, want scaffolded schema", string(schemaData))
	}

	devEnvData, err := os.ReadFile(filepath.Join(root, "env/api/dev.env"))
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if string(devEnvData) != "APP_ENV=dev\n" {
		t.Fatalf("dev env file = %q, want APP_ENV sample", string(devEnvData))
	}

	sopsData, err := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if err != nil {
		t.Fatalf("read sops file: %v", err)
	}
	if string(sopsData) != "creation_rules:\n  - path_regex: ^env/.*\\.env$\n    age: []\n" {
		t.Fatalf("sops file = %q, want scaffolded sops config", string(sopsData))
	}
}

func TestInit_ExistingFiles(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": "existing\n",
	})

	// Act
	_, err := app.Init(t.Context(), nil, app.InitOptions{
		ConfigPath: filepath.Join(root, "envdesk.yaml"),
	})

	// Assert
	if err == nil {
		t.Fatal("Init() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `check scaffold target "envdesk.yaml": file already exists`) {
		t.Fatalf("Init() error = %q, want existing file error", err)
	}

	configData, readErr := os.ReadFile(filepath.Join(root, "envdesk.yaml"))
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	if string(configData) != "existing\n" {
		t.Fatalf("config file = %q, want original content", string(configData))
	}
}

func TestInit_ForceOverwrite(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml":        "existing\n",
		"env.schema/api.yaml": "existing\n",
		"env/api/dev.env":     "OLD=1\n",
	})

	// Act
	result, err := app.Init(t.Context(), nil, app.InitOptions{
		ConfigPath: filepath.Join(root, "envdesk.yaml"),
		Force:      true,
	})
	// Assert
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	if result.Files[0].Action != "overwrote" {
		t.Fatalf("result.Files[0].Action = %q, want overwrote", result.Files[0].Action)
	}

	devEnvData, readErr := os.ReadFile(filepath.Join(root, "env/api/dev.env"))
	if readErr != nil {
		t.Fatalf("read env file: %v", readErr)
	}
	if string(devEnvData) != "APP_ENV=dev\n" {
		t.Fatalf("dev env file = %q, want overwritten scaffold content", string(devEnvData))
	}
}
