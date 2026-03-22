package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestStatusCommand_JSONOutput(t *testing.T) {
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
    type: enum
    values: [dev, stg]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
`,
		"env/api/dev.env": "APP_ENV=local\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=postgres://stg.example.com/app\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"status",
		"--json",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}

	var result app.StatusResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}
	if result.Summary.LintErrorCount != 2 {
		t.Fatalf("result.Summary.LintErrorCount = %d, want 2", result.Summary.LintErrorCount)
	}
	if result.Summary.DriftIssueCount != 1 {
		t.Fatalf("result.Summary.DriftIssueCount = %d, want 1", result.Summary.DriftIssueCount)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(result.Rows) = %d, want 2", len(result.Rows))
	}
}

func TestStatusCommand_VerboseOutput(t *testing.T) {
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
    type: enum
    values: [dev, stg]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
`,
		"env/api/dev.env": "APP_ENV=local\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=postgres://stg.example.com/app\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"--verbose",
		"status",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "SERVICE") {
		t.Fatalf("stdout = %q, want header", stdout.String())
	}
	if !strings.Contains(stdout.String(), "PATH") {
		t.Fatalf("stdout = %q, want verbose path column", stdout.String())
	}
	if !strings.Contains(stdout.String(), "env/api/dev.env") {
		t.Fatalf("stdout = %q, want env path", stdout.String())
	}
	if !strings.Contains(stdout.String(), "drift issues:1 error:1 missing:1") {
		t.Fatalf("stdout = %q, want sync summary", stdout.String())
	}
	if !strings.Contains(stdout.String(), "summary: 1 services, 2 environments") {
		t.Fatalf("stdout = %q, want summary line", stdout.String())
	}
}

func TestStatusCommand_WarningsOnlySucceeds(t *testing.T) {
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
    type: enum
    values: [dev, stg]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
		"env/api/stg.env": "APP_ENV=stg\nEXTRA_FLAG=2\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"status",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "warn:1") {
		t.Fatalf("stdout = %q, want warning status", stdout.String())
	}
	if strings.Contains(stdout.String(), "all environments look healthy") {
		t.Fatalf("stdout = %q, should not report fully healthy when warnings exist", stdout.String())
	}
}
