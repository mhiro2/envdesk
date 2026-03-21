package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestExampleGenerate_MergesSchemaAndEnvStructure(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: string
    secret: false
  DATABASE_URL:
    required: true
    type: string
    secret: true
`,
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	decryptData := map[string][]byte{
		filepath.Join(root, "env/api/dev.env"): []byte("APP_ENV=dev\nLEGACY_FLAG=1\n"),
		filepath.Join(root, "env/api/stg.env"): []byte("APP_ENV=stg\nSTG_ONLY=yes\n"),
	}
	var decryptPaths []string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPaths = append(decryptPaths, path)
			data, ok := decryptData[path]
			if !ok {
				return nil, fmt.Errorf("missing decrypt data for %q", path)
			}
			return data, nil
		},
	}

	// Act
	result, err := app.ExampleGenerate(t.Context(), project, adapter, app.ExampleGenerateOptions{})
	// Assert
	if err != nil {
		t.Fatalf("ExampleGenerate() error = %v, want nil", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}
	if result.Files[0].Action != "created" {
		t.Fatalf("result.Files[0].Action = %q, want created", result.Files[0].Action)
	}

	outputPath := filepath.Join(root, "env/api/.env.example")
	if result.Files[0].Path != outputPath {
		t.Fatalf("result.Files[0].Path = %q, want %q", result.Files[0].Path, outputPath)
	}

	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read example file: %v", readErr)
	}
	if string(data) != "# required, type=string, secret=false\nAPP_ENV=\n# required, type=string, secret=true\nDATABASE_URL=\nLEGACY_FLAG=\nSTG_ONLY=\n" {
		t.Fatalf("example file = %q, want merged keys without values", string(data))
	}
	if len(decryptPaths) != 2 {
		t.Fatalf("len(decryptPaths) = %d, want 2", len(decryptPaths))
	}
}

func TestExampleGenerate_UsesServiceFilterAndOut(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
  - name: web
    files:
      dev: env/web/dev.env
`,
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	outPath := filepath.Join(root, "docs", "api.env.example")
	decryptData := map[string][]byte{
		filepath.Join(root, "env/api/dev.env"): []byte("APP_ENV=dev\nAPI_URL=https://dev.example.com\n"),
	}
	var decryptPaths []string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPaths = append(decryptPaths, path)
			data, ok := decryptData[path]
			if !ok {
				return nil, fmt.Errorf("missing decrypt data for %q", path)
			}
			return data, nil
		},
	}

	// Act
	result, err := app.ExampleGenerate(t.Context(), project, adapter, app.ExampleGenerateOptions{
		Service: "api",
		Out:     outPath,
	})
	// Assert
	if err != nil {
		t.Fatalf("ExampleGenerate() error = %v, want nil", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}
	if result.Files[0].Path != outPath {
		t.Fatalf("result.Files[0].Path = %q, want %q", result.Files[0].Path, outPath)
	}

	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read example file: %v", readErr)
	}
	if string(data) != "APP_ENV=\nAPI_URL=\n" {
		t.Fatalf("example file = %q, want api example", string(data))
	}
	if len(decryptPaths) != 1 {
		t.Fatalf("len(decryptPaths) = %d, want 1", len(decryptPaths))
	}
}

func TestExampleGenerate_RefusesOverwriteWithoutForce(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	outPath := filepath.Join(root, "env", "api", ".env.example")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		t.Fatalf("mkdir example dir: %v", err)
	}
	if err := os.WriteFile(outPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("write example file: %v", err)
	}

	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			if path != filepath.Join(root, "env/api/dev.env") {
				return nil, fmt.Errorf("missing decrypt data for %q", path)
			}
			return []byte("APP_ENV=dev\n"), nil
		},
	}

	// Act
	_, err = app.ExampleGenerate(t.Context(), project, adapter, app.ExampleGenerateOptions{
		Service: "api",
		Out:     outPath,
	})

	// Assert
	if err == nil {
		t.Fatal("ExampleGenerate() error = nil, want non-nil")
	}
	if err.Error() != fmt.Sprintf(`write example file for api: check example target %q: file already exists`, outPath) {
		t.Fatalf("ExampleGenerate() error = %q, want overwrite protection", err.Error())
	}

	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read example file: %v", readErr)
	}
	if string(data) != "existing\n" {
		t.Fatalf("example file = %q, want original content", string(data))
	}
}

func TestExampleGenerate_FallsBackToSchemaOnDecryptFailure(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: string
    secret: false
  DATABASE_URL:
    required: true
    type: string
    secret: true
`,
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return nil, fmt.Errorf("sops: key not found")
		},
	}

	// Act
	result, err := app.ExampleGenerate(t.Context(), project, adapter, app.ExampleGenerateOptions{
		Service: "api",
	})
	// Assert
	if err != nil {
		t.Fatalf("ExampleGenerate() error = %v, want nil (schema fallback)", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}

	data, readErr := os.ReadFile(result.Files[0].Path)
	if readErr != nil {
		t.Fatalf("read example file: %v", readErr)
	}
	if string(data) != "# required, type=string, secret=false\nAPP_ENV=\n# required, type=string, secret=true\nDATABASE_URL=\n" {
		t.Fatalf("example file = %q, want schema-only keys", string(data))
	}
}

func TestExampleGenerate_FailsWithoutSchemaOnDecryptFailure(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return nil, fmt.Errorf("sops: key not found")
		},
	}

	// Act
	_, err = app.ExampleGenerate(t.Context(), project, adapter, app.ExampleGenerateOptions{
		Service: "api",
	})

	// Assert
	if err == nil {
		t.Fatal("ExampleGenerate() error = nil, want non-nil (no schema fallback available)")
	}
}
