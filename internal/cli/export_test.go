package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/platformtest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_ExportWritesFile(t *testing.T) {
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

	var decryptPath string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPath = path
			return []byte("APP_ENV=dev\n"), nil
		},
	}
	setupCryptoAdapter(t, adapter)

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	outPath := filepath.Join(root, ".env.local")
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"export",
		"api",
		"dev",
		"--out", outPath,
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if decryptPath != filepath.Join(root, "env/api/dev.env") {
		t.Fatalf("Decrypt() path = %q, want target env file", decryptPath)
	}

	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read export file: %v", readErr)
	}
	if string(data) != "APP_ENV=dev\n" {
		t.Fatalf("export file = %q, want decrypted content", string(data))
	}

	info, statErr := os.Stat(outPath)
	if statErr != nil {
		t.Fatalf("stat export file: %v", statErr)
	}
	if !platformtest.SupportsExactFileModes() {
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("export file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestRootCommand_ExportWritesStdout(t *testing.T) {
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

	setupCryptoAdapter(t, &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return []byte("APP_ENV=dev\n"), nil
		},
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"export",
		"api",
		"dev",
		"--stdout",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "APP_ENV=dev\n" {
		t.Fatalf("stdout = %q, want decrypted content", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning:") {
		t.Fatalf("stderr = %q, want warning message", stderr.String())
	}
	if _, statErr := os.Stat(filepath.Join(root, ".env.local")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected export file state: %v", statErr)
	}
}

func TestRootCommand_ExportRefusesOverwrite(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		".env.local":      "existing\n",
	})

	setupCryptoAdapter(t, &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return []byte("APP_ENV=dev\n"), nil
		},
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	outPath := filepath.Join(root, ".env.local")
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"export",
		"api",
		"dev",
		"--out", outPath,
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `check export target`) {
		t.Fatalf("Execute() error = %q, want overwrite protection", err.Error())
	}
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read export file: %v", readErr)
	}
	if string(data) != "existing\n" {
		t.Fatalf("export file = %q, want original content", string(data))
	}
}

func TestNewRootCommand_RegistersExport(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"export"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want export command")
	}
}
