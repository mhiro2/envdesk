package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_DiffCiSummaryFailsOnChanges(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDEV_ONLY=1\n",
		"env/api/stg.env": "APP_ENV=stg\nEXTRA_FLAG=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "found 3 changes") {
		t.Fatalf("Execute() error = %q, want diff failure", err.Error())
	}
	if stdout.String() != "api dev..stg: 3 changes (1 added, 1 removed, 1 modified, 1 rename candidates)\n" {
		t.Fatalf("stdout = %q, want concise summary", stdout.String())
	}
}

func TestRootCommand_DiffCiSummaryReportsNoChanges(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
		"env/api/stg.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "api dev..stg: no changes\n" {
		t.Fatalf("stdout = %q, want no changes summary", stdout.String())
	}
}

func TestRootCommand_DiffCiSummaryWarnsOnSummaryWriteErrorWhenVerbose(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
		"env/api/stg.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
	})
	t.Setenv("GITHUB_STEP_SUMMARY", t.TempDir())

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"--verbose",
		"diff",
		"api",
		"dev",
		"stg",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.String() != "api dev..stg: no changes\n" {
		t.Fatalf("stdout = %q, want no changes summary", stdout.String())
	}
	if !strings.Contains(stderr.String(), "skip GitHub summary") {
		t.Fatalf("stderr = %q, want summary warning", stderr.String())
	}
}

func TestCompleteEnvironmentFlag_UsesServiceFilterWhenPresent(t *testing.T) {
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
      prod: env/web/prod.env
`,
	})

	cmd := &cobra.Command{Use: "lint"}
	cmd.Flags().String("config", filepath.Join(root, "envdesk.yaml"), "")
	cmd.Flags().String("service", "api", "")

	// Act
	envs, directive := completeEnvironmentFlag(cmd, nil, "")

	// Assert
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if len(envs) != 2 || envs[0] != "dev" || envs[1] != "stg" {
		t.Fatalf("envs = %v, want [dev stg]", envs)
	}
}

func TestCompleteEnvironmentFlag_ReturnsUniqueEnvironmentsWithoutServiceFilter(t *testing.T) {
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
      prod: env/web/prod.env
`,
	})

	cmd := &cobra.Command{Use: "lint"}
	cmd.Flags().String("config", filepath.Join(root, "envdesk.yaml"), "")
	cmd.Flags().String("service", "", "")

	// Act
	envs, directive := completeEnvironmentFlag(cmd, nil, "")

	// Assert
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if len(envs) != 3 || envs[0] != "dev" || envs[1] != "prod" || envs[2] != "stg" {
		t.Fatalf("envs = %v, want [dev prod stg]", envs)
	}
}

func TestRootCommand_DiffJSONIncludesSummary(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDEV_ONLY=1\n",
		"env/api/stg.env": "APP_ENV=stg\nEXTRA_FLAG=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--json",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	var result app.DiffResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.Summary.Total != 3 || result.Summary.Added != 1 || result.Summary.Removed != 1 || result.Summary.Modified != 1 || result.Summary.Renamed != 1 {
		t.Fatalf("result.Summary = %#v, want total=3, 1 added, 1 removed, 1 modified, 1 renamed", result.Summary)
	}
}

func TestRootCommand_DiffCiSummaryIncludesRenameCandidates(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nLEGACY_FLAG=shared\nKEEP=1\n",
		"env/api/stg.env": "APP_ENV=dev\nNEW_FLAG=shared\nKEEP=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--ci-summary",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "1 rename candidates") {
		t.Fatalf("stdout = %q, want rename candidate summary", stdout.String())
	}
}

func TestRootCommand_DiffShowMetadataIncludesFindings(t *testing.T) {
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
		"env/api/dev.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
		"env/api/stg.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
	})

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--show-metadata",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "missing required key") {
		t.Fatalf("stdout = %q, want schema finding", stdout.String())
	}
	if !strings.Contains(stdout.String(), "key not declared in schema") {
		t.Fatalf("stdout = %q, want undeclared key warning", stdout.String())
	}
}

func TestRootCommand_DiffJSONIncludesSummaryForEncryptedEnv(t *testing.T) {
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
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nDEV_ONLY=1\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\nEXTRA_FLAG=1\n"),
	})

	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--json",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	var result app.DiffResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.Summary.Total != 3 || result.Summary.Added != 1 || result.Summary.Removed != 1 || result.Summary.Modified != 1 {
		t.Fatalf("result.Summary = %#v, want total=3, 1 added, 1 removed, 1 modified", result.Summary)
	}
}
