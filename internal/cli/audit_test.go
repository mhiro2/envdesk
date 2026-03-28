package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
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
	for _, want := range []string{"--service", "--env", "--key", "--json", "schema metadata", "drift"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q in %q", want, output)
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
		t.Fatalf("error = %q, want config error", err.Error())
	}
}

func TestAuditCommand_UnknownEnvironment(t *testing.T) {
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

	cmd := newPlaintextRootCommand(t)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", root + "/envdesk.yaml",
		"audit",
		"--env", "prod",
	})

	// Act & Assert
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `select environment "prod": not configured`) {
		t.Fatalf("error = %q, want unknown environment", err.Error())
	}
}

func TestWriteAuditTable_IncludesSchemaAndDrift(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{}
	cmd.Flags().Bool("verbose", false, "")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Flags().Set("verbose", "true"); err != nil {
		t.Fatalf("set verbose flag: %v", err)
	}

	results := []app.AuditResult{
		{
			Service:     "api",
			Environment: "dev",
			Path:        "env/api/dev.env",
			Entries: []app.AuditEntry{
				{
					Key:     "DATABASE_URL",
					Present: true,
					LastValueChange: &app.AuditFact{
						Author:    "Alice",
						Date:      time.Date(2023, 11, 15, 12, 0, 0, 0, time.UTC),
						CommitSHA: "a1b2c3d4e5f6a7b8",
						Summary:   "feat: add initial env",
					},
					Schema: &app.AuditSchemaState{
						Required: true,
						Secret:   true,
						Type:     "url",
					},
					LastSchemaChange: &app.AuditFact{
						Author:    "Bob",
						Date:      time.Date(2023, 11, 16, 12, 0, 0, 0, time.UTC),
						CommitSHA: "b2c3d4e5f6a7b8c9",
						Summary:   "feat: tighten database schema",
					},
					Drift: app.AuditDriftState{
						State: "drift",
						Kind:  app.SyncIssueKindRequired,
						Since: &app.AuditFact{
							Author:    "Carol",
							Date:      time.Date(2023, 11, 17, 12, 0, 0, 0, time.UTC),
							CommitSHA: "c3d4e5f6a7b8c9d0",
							Summary:   "feat: introduce drift",
						},
					},
				},
			},
		},
	}

	// Act
	err := writeAuditTable(cmd, results)
	if err != nil {
		t.Fatalf("writeAuditTable() error = %v, want nil", err)
	}

	// Assert
	output := stdout.String()
	for _, want := range []string{"DATABASE_URL", "required", "url", "2023-11-15", "a1b2c3d4", "required since 2023-11-17"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q in %q", want, output)
		}
	}
}
