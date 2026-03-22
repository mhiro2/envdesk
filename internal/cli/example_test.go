package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_ExampleGenerateWritesFile(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
	})

	var decryptPaths []string
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(_ context.Context, path string) ([]byte, error) {
			decryptPaths = append(decryptPaths, path)
			if path != filepath.Join(root, "env/api/dev.env") {
				return nil, fmt.Errorf("missing decrypt data for %q", path)
			}
			return []byte("APP_ENV=dev\nAPI_URL=https://dev.example.com\n"), nil
		},
	}
	outPath := filepath.Join(root, "docs", "api.env.example")
	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"example",
		"generate",
		"--service", "api",
		"--out", outPath,
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "created docs/api.env.example\n" {
		t.Fatalf("stdout = %q, want created example output", stdout.String())
	}

	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read example file: %v", readErr)
	}
	if string(data) != "APP_ENV=\nAPI_URL=\n" {
		t.Fatalf("example file = %q, want generated example", string(data))
	}
	if len(decryptPaths) != 1 {
		t.Fatalf("len(decryptPaths) = %d, want 1", len(decryptPaths))
	}
}

func TestNewRootCommand_RegistersExampleGenerate(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"example", "generate"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want example generate command")
	}
	if !strings.EqualFold(found.Name(), "generate") {
		t.Fatalf("Find() command = %q, want generate", found.Name())
	}
}
