package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_SyncKeysDryRunReportsTargets(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=\nFEATURE_FLAG=\n",
		"env/api/stg.env": "APP_ENV=stg\nLEGACY_FLAG=true\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"sync-keys",
		"api",
		"dev",
		"--to", "stg",
		"--dry-run",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "stg: +DATABASE_URL,FEATURE_FLAG -LEGACY_FLAG (dry-run)\n" {
		t.Fatalf("stdout = %q, want dry-run summary", stdout.String())
	}
}

func TestRootCommand_SyncKeysRejectsDuplicateTargets(t *testing.T) {
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

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"sync-keys",
		"api",
		"dev",
		"--to", "stg,stg",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate target") {
		t.Fatalf("Execute() error = %q, want duplicate target failure", err.Error())
	}
}

func TestRootCommand_SyncKeysWritesEncryptedOutput(t *testing.T) {
	// Arrange
	adapter := &cryptotest.FakeEncryptAdapter{}
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nDATABASE_URL=\nFEATURE_FLAG=\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\nLEGACY_FLAG=true\n"),
	})

	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"sync-keys",
		"api",
		"dev",
		"--to", "stg",
		"--placeholders",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "stg: +DATABASE_URL,FEATURE_FLAG -LEGACY_FLAG\n" {
		t.Fatalf("stdout = %q, want sync summary", stdout.String())
	}

	targetPath := filepath.Join(root, "env/api/stg.env")
	encrypted, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read synced file: %v", readErr)
	}
	decrypted, decryptErr := adapter.Decrypt(t.Context(), targetPath)
	if decryptErr != nil {
		t.Fatalf("decrypt synced file: %v", decryptErr)
	}
	if string(encrypted) == string(decrypted) {
		t.Fatal("synced file should remain encrypted on disk")
	}
	if string(decrypted) != "APP_ENV=stg\nDATABASE_URL=\nFEATURE_FLAG=\n" {
		t.Fatalf("decrypted synced file = %q, want normalized placeholder output", string(decrypted))
	}
}
