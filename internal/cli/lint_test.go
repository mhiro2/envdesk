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

func TestLintCommand_JSONIncludesCounts(t *testing.T) {
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
  DATABASE_URL:
    required: true
    type: url
    secret: true
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=local\nEXTRA_FLAG=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"lint",
		"--json",
		"--service", "api",
		"--env", "dev",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}

	var result app.LintResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.ErrorCount != 2 {
		t.Fatalf("result.ErrorCount = %d, want 2", result.ErrorCount)
	}
	if result.WarningCount != 1 {
		t.Fatalf("result.WarningCount = %d, want 1", result.WarningCount)
	}
	if len(result.Problems) != 3 {
		t.Fatalf("len(result.Problems) = %d, want 3", len(result.Problems))
	}
}

func TestLintCommand_StrictTreatsWarningsAsErrors(t *testing.T) {
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
`,
		"env/api/dev.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"lint",
		"--strict",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "found 0 errors and 1 warnings") {
		t.Fatalf("Execute() error = %q, want strict warning failure", err.Error())
	}
	if !strings.Contains(stdout.String(), "warning") {
		t.Fatalf("stdout = %q, want warning output", stdout.String())
	}
}

func TestLintCommand_FailsOnMalformedSchema(t *testing.T) {
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
    values: [dev]
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"lint",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "values require enum type") {
		t.Fatalf("Execute() error = %q, want schema validation failure", err.Error())
	}
}

func TestLintCommand_FailsOnEnvParseError(t *testing.T) {
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
`,
		"env/api/dev.env": "APP_ENV\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"lint",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse env file") {
		t.Fatalf("Execute() error = %q, want parse env file failure", err.Error())
	}
}

func TestLintCommand_EncryptedJSONIncludesCounts(t *testing.T) {
	// Arrange
	adapter := &cryptotest.FakeEncryptAdapter{}
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
		"env.schema/api.yaml": `keys:
  DATABASE_URL:
    required: true
    type: url
    secret: true
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
`,
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=local\nEXTRA_FLAG=1\n"),
	})

	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"lint",
		"--json",
		"--service", "api",
		"--env", "dev",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}

	var result app.LintResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.ErrorCount != 2 {
		t.Fatalf("result.ErrorCount = %d, want 2", result.ErrorCount)
	}
	if result.WarningCount != 1 {
		t.Fatalf("result.WarningCount = %d, want 1", result.WarningCount)
	}
}
