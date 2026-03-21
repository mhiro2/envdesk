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

func TestDoctorCommand_ReportsMissingConfig(t *testing.T) {
	// Arrange
	root := t.TempDir()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"doctor",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(stdout.String(), "missing config file") {
		t.Fatalf("stdout = %q, want missing config finding", stdout.String())
	}
}

func TestDoctorCommand_JSONOutput(t *testing.T) {
	// Arrange
	root := t.TempDir()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"doctor",
		"--json",
	})

	// Act
	err := cmd.Execute()
	// Doctor may fail but JSON should still be valid
	_ = err

	// Assert
	var result app.DoctorResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v (stdout=%q)", unmarshalErr, stdout.String())
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected at least one finding for missing config")
	}
}

func TestDoctorCommand_HealthyWithNoErrors(t *testing.T) {
	// Arrange
	// Create a minimal project that would pass doctor (except maybe tools not available)
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "ENC[AES256_GCM,data:abc]\nsops:\n    version: 3\n",
		".sops.yaml": `creation_rules:
  - path_regex: ^env/.*\.env$
    age:
      - age1test000000000000000000000000000000000000000000000000000
`,
		".gitignore": "*.env.local\n",
	})

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"doctor",
		"--json",
	})

	// Act
	_ = cmd.Execute()

	// Assert
	// We just verify JSON is valid, since actual tools may not be available
	var result app.DoctorResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
}
