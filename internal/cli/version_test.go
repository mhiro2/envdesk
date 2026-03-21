package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/cli"
)

func TestRootCommand_VersionPrintsEmbeddedVersion(t *testing.T) {
	// Arrange
	cli.SetBuildInfo("1.2.3", "", "")
	t.Cleanup(func() {
		cli.ResetBuildInfo()
	})

	cmd := cli.NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--version"})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "1.2.3") {
		t.Fatalf("stdout = %q, want embedded version", stdout.String())
	}
}
