package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestEditCommand_RequiresTwoArgs(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"edit", "api"})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "accepts 2 arg(s)") {
		t.Fatalf("Execute() error = %q, want arg count error", err.Error())
	}
}

func TestEditCommand_FailsOnMissingConfig(t *testing.T) {
	// Arrange
	root := t.TempDir()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"edit", "api", "dev",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "load project config") {
		t.Fatalf("Execute() error = %q, want load project config failure", err.Error())
	}
}

func TestEditCommand_FailsOnUnknownService(t *testing.T) {
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

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"edit", "nonexistent", "dev",
	})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Execute() error = %q, want service not found", err.Error())
	}
}

func TestNewRootCommand_RegistersEdit(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"edit"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want edit command")
	}
}
