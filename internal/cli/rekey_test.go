package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_RekeyDryRun(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	var rekeyPaths []string
	adapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			rekeyPaths = append(rekeyPaths, path)
			return nil
		},
	}
	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"rekey",
		"--service", "api",
		"--env", "stg",
		"--dry-run",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "[1/1] rekeyed env/api/stg.env (dry-run)\n" {
		t.Fatalf("stdout = %q, want dry-run output", stdout.String())
	}
	if len(rekeyPaths) != 0 {
		t.Fatalf("len(adapter.rekeyPaths) = %d, want 0", len(rekeyPaths))
	}
	if _, statErr := os.Stat(filepath.Join(root, "env/api/stg.env")); statErr != nil {
		t.Fatalf("stat env file: %v", statErr)
	}
}

func TestRootCommand_RekeyJSON(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	var rekeyPaths []string
	adapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			rekeyPaths = append(rekeyPaths, path)
			return nil
		},
	}
	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"rekey",
		"--service", "api",
		"--json",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	var result app.RekeyResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if len(result.Files) != 2 {
		t.Fatalf("len(result.Files) = %d, want 2", len(result.Files))
	}
	if len(rekeyPaths) != 2 {
		t.Fatalf("len(adapter.rekeyPaths) = %d, want 2", len(rekeyPaths))
	}
}

func TestRootCommand_RekeyJSON_ReturnsErrorAfterWritingResult(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	adapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			if filepath.Base(path) == "stg.env" {
				return errors.New("mock rekey failure")
			}

			return nil
		},
	}
	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"rekey",
		"--service", "api",
		"--json",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want partial failure error")
	}
	if err.Error() != "rekey env files: rekey env files: 1 of 2 files failed" {
		t.Fatalf("Execute() error = %q, want wrapped partial failure", err.Error())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var result app.RekeyResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(result.Errors) = %d, want 1", len(result.Errors))
	}
	if result.Errors[0].Env != "stg" {
		t.Fatalf("result.Errors[0].Env = %q, want stg", result.Errors[0].Env)
	}
}

func TestNewRootCommand_RegistersRekey(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"rekey"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want rekey command")
	}
}
