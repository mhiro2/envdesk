package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestExport_DecryptedContent(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	var decryptPath string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPath = path
			return []byte("APP_ENV=dev\n"), nil
		},
	}

	// Act
	plaintext, err := app.Export(t.Context(), project, adapter, "api", "dev")
	// Assert
	if err != nil {
		t.Fatalf("Export() error = %v, want nil", err)
	}
	if string(plaintext) != "APP_ENV=dev\n" {
		t.Fatalf("Export() = %q, want decrypted content", string(plaintext))
	}
	if decryptPath != filepath.Join(root, "env/api/dev.env") {
		t.Fatalf("Decrypt() path = %q, want target env file", decryptPath)
	}
}

func TestExport_ValidationErrors(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	tests := []struct {
		name    string
		service string
		env     string
		wantErr string
	}{
		{
			name:    "missing service",
			service: "web",
			env:     "dev",
			wantErr: `export env file: lookup service "web": not found`,
		},
		{
			name:    "missing environment",
			service: "api",
			env:     "stg",
			wantErr: `lookup env file for api/stg: lookup environment "stg" for service "api": not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			_, err := app.Export(t.Context(), project, &cryptotest.StubAdapter{}, tt.service, tt.env)

			// Assert
			if err == nil {
				t.Fatal("Export() error = nil, want non-nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Export() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExport_DecryptError(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	var decryptPath string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPath = path
			return nil, errors.New("boom")
		},
	}

	// Act
	_, err = app.Export(t.Context(), project, adapter, "api", "dev")

	// Assert
	if err == nil {
		t.Fatal("Export() error = nil, want non-nil")
	}
	if err.Error() != `decrypt env file for api/dev: boom` {
		t.Fatalf("Export() error = %q, want decrypt error", err.Error())
	}
	if decryptPath != filepath.Join(root, "env/api/dev.env") {
		t.Fatalf("Decrypt() path = %q, want target env file", decryptPath)
	}
}

func TestExport_MissingAdapter(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	_, err = app.Export(t.Context(), project, nil, "api", "dev")

	// Assert
	if err == nil {
		t.Fatal("Export() error = nil, want non-nil")
	}
	if err.Error() != "export env file: missing crypto adapter" {
		t.Fatalf("Export() error = %q, want missing adapter", err.Error())
	}
}
