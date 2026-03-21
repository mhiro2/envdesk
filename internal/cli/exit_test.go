package cli

import (
	"errors"
	"testing"
)

func TestExitCode_UsesWrappedCode(t *testing.T) {
	// Arrange
	err := withExitCode(errors.New("validate env files: found 1 errors and 0 warnings"))

	// Act
	code := ExitCode(err)

	// Assert
	if code != ExitCodeCheckFailed {
		t.Fatalf("ExitCode(err) = %d, want %d", code, ExitCodeCheckFailed)
	}
}

func TestExitCode_UsesRuntimeCodeByDefault(t *testing.T) {
	// Arrange
	err := errors.New("load project config: boom")

	// Act
	code := ExitCode(err)

	// Assert
	if code != ExitCodeRuntime {
		t.Fatalf("ExitCode(err) = %d, want %d", code, ExitCodeRuntime)
	}
}
