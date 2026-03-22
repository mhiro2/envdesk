package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestCheckSyncCommand_JSONOutput(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--json",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	var issues []app.SyncIssue
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &issues); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Key != "PAYMENT_TIMEOUT" {
		t.Fatalf("issues[0].Key = %q, want PAYMENT_TIMEOUT", issues[0].Key)
	}
}

func TestCheckSyncCommand_ServiceFilter(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
  - name: web
    files:
      dev: env/web/dev.env
      stg: env/web/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env": "APP_ENV=stg\n",
		"env/web/dev.env": "APP_ENV=dev\n",
		"env/web/stg.env": "APP_ENV=stg\nEXTRA=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--service", "web",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if strings.Contains(stdout.String(), "PAYMENT_TIMEOUT") {
		t.Fatalf("stdout = %q, should not contain api drift", stdout.String())
	}
	if !strings.Contains(stdout.String(), "EXTRA") {
		t.Fatalf("stdout = %q, want web drift", stdout.String())
	}
}

func TestCheckSyncCommand_NoDrift(t *testing.T) {
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
		"check-sync",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "all environments are in sync") {
		t.Fatalf("stdout = %q, want no drift", stdout.String())
	}
}

func TestCheckSyncCommand_FailsOnMissingConfig(t *testing.T) {
	// Arrange
	root := t.TempDir()

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
}

func TestCheckSyncCommand_ReportsDrift(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "PAYMENT_TIMEOUT missing in stg") {
		t.Fatalf("stdout = %q, want PAYMENT_TIMEOUT drift", stdout.String())
	}
}

func TestCheckSyncCommand_CiSummaryReportsDrift(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
`,
		"env/api/dev.env":  "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env":  "APP_ENV=stg\n",
		"env/api/prod.env": "APP_ENV=prod\nPAYMENT_TIMEOUT=60\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "api: 1 drift issues across dev, prod, stg") {
		t.Fatalf("stdout = %q, want concise drift summary", stdout.String())
	}
}

func TestCheckSyncCommand_CiSummaryReportsNoDrift(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env": "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "all environments are in sync\n" {
		t.Fatalf("stdout = %q, want no drift summary", stdout.String())
	}
}

func TestCheckSyncCommand_JSONOutputForEncryptedEnv(t *testing.T) {
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
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nPAYMENT_TIMEOUT=30\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\n"),
	})

	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--json",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	var issues []app.SyncIssue
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &issues); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Key != "PAYMENT_TIMEOUT" {
		t.Fatalf("issues[0].Key = %q, want PAYMENT_TIMEOUT", issues[0].Key)
	}
}

func TestNewRootCommand_RegistersCheckSync(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"check-sync"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want check-sync command")
	}
}
