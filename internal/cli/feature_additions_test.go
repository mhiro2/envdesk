package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_DiffValueModePublic_HidesSecretValues(t *testing.T) {
	// Arrange
	setupPlaintextAdapter(t)
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
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=https://dev.example.com\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n",
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"diff",
		"api",
		"dev",
		"stg",
		"--value-mode", "public",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), `APP_ENV: "dev" -> "stg"`) {
		t.Fatalf("stdout = %q, want public value diff", stdout.String())
	}
	if !strings.Contains(stdout.String(), `DATABASE_URL: "(secret hidden)" -> "(secret hidden)"`) {
		t.Fatalf("stdout = %q, want hidden secret diff", stdout.String())
	}
}

func TestCheckSyncCommand_StrictRequiredOnly_FiltersOutput(t *testing.T) {
	// Arrange
	setupPlaintextAdapter(t)
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg, prod]
    secret: false
  REQUIRED_TIMEOUT:
    required: true
    type: int
    secret: false
  OPTIONAL_FLAG:
    required: false
    type: bool
    secret: false
`,
		"env/api/dev.env":  "APP_ENV=dev\nREQUIRED_TIMEOUT=30\nOPTIONAL_FLAG=true\n",
		"env/api/stg.env":  "APP_ENV=stg\n",
		"env/api/prod.env": "APP_ENV=prod\nREQUIRED_TIMEOUT=45\n",
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"check-sync",
		"--strict-required-only",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "REQUIRED_TIMEOUT") {
		t.Fatalf("stdout = %q, want required drift", stdout.String())
	}
	if strings.Contains(stdout.String(), "OPTIONAL_FLAG") {
		t.Fatalf("stdout = %q, want optional drift filtered", stdout.String())
	}
}

func TestRootCommand_MemberDryRun_DoesNotWriteConfig(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: []
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
		"alice.pub":       "age1aliceexample\n",
	})

	setupCryptoAdapter(t, &cryptotest.StubAdapter{
		RekeyFunc: func(context.Context, string) error {
			t.Fatal("Rekey() should not be called during dry-run")
			return nil
		},
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"member",
		"add",
		filepath.Join(root, "alice.pub"),
		"--scope", "api",
		"--dry-run",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "(dry-run)") {
		t.Fatalf("stdout = %q, want dry-run preview", stdout.String())
	}
	data, readErr := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if readErr != nil {
		t.Fatalf("read sops config: %v", readErr)
	}
	if strings.Contains(string(data), "age1aliceexample") {
		t.Fatalf("sops config = %q, want unchanged file during dry-run", string(data))
	}
}
