package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAuditCommand_HelpOutput(t *testing.T) {
	// Arrange
	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"audit", "--help"})

	// Act
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	// Assert
	output := stdout.String()
	for _, want := range []string{"--service", "--env", "--key", "--json", "git blame"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestAuditCommand_MissingConfig(t *testing.T) {
	// Arrange
	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", "/nonexistent/envdesk.yaml",
		"audit",
	})

	// Act & Assert
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "read config") {
		t.Errorf("error = %q, want config error", err.Error())
	}
}
