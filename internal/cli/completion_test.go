package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRootCommand_RegistersCompletion(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"completion"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want completion command")
	}
}

func TestCompletionCommand_Bash(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"completion", "bash"})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "bash") {
		t.Fatal("expected bash completion output")
	}
}

func TestCompletionCommand_Zsh(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"completion", "zsh"})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected zsh completion output")
	}
}

func TestCompletionCommand_Fish(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"completion", "fish"})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected fish completion output")
	}
}

func TestCompletionCommand_Powershell(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"completion", "powershell"})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected powershell completion output")
	}
}

func TestCompletionCommand_InvalidShell(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"completion", "invalid"})

	// Act
	err := cmd.Execute()

	// Assert
	if err == nil {
		t.Fatal("Execute() error = nil, want error for invalid shell")
	}
}
