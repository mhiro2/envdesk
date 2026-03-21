package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/cli"
)

func TestRootCommand_InitScaffoldsProject(t *testing.T) {
	// Arrange
	root := t.TempDir()

	cmd := cli.NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"init",
		"--services", "api,web",
		"--envs", "dev,prod",
		"--sops",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "created envdesk.yaml") {
		t.Fatalf("stdout = %q, want created envdesk.yaml", stdout.String())
	}

	configData, readErr := os.ReadFile(filepath.Join(root, "envdesk.yaml"))
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	if !strings.Contains(string(configData), "name: web") {
		t.Fatalf("config file = %q, want web service", string(configData))
	}

	prodData, readErr := os.ReadFile(filepath.Join(root, "env/web/prod.env"))
	if readErr != nil {
		t.Fatalf("read env file: %v", readErr)
	}
	if string(prodData) != "APP_ENV=prod\n" {
		t.Fatalf("prod env file = %q, want APP_ENV sample", string(prodData))
	}
}
