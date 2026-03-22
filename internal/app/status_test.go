package app_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestStatus_AggregatesLintSyncAndUpdatedAt(t *testing.T) {
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
		"env/api/dev.env": "APP_ENV=local\nEXTRA_FLAG=1\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=postgres://stg.example.com/app\n",
	})

	devPath := filepath.Join(root, "env/api/dev.env")
	stgPath := filepath.Join(root, "env/api/stg.env")
	devUpdatedAt := time.Date(2026, 3, 20, 10, 30, 0, 0, time.Local)
	stgUpdatedAt := time.Date(2026, 3, 21, 11, 45, 0, 0, time.Local)
	if err := os.Chtimes(devPath, devUpdatedAt, devUpdatedAt); err != nil {
		t.Fatalf("chtimes dev env: %v", err)
	}
	if err := os.Chtimes(stgPath, stgUpdatedAt, stgUpdatedAt); err != nil {
		t.Fatalf("chtimes stg env: %v", err)
	}

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Status(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.StatusOptions{})
	// Assert
	if err != nil {
		t.Fatalf("status env files: %v", err)
	}
	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}
	if result.Summary.ServiceCount != 1 {
		t.Fatalf("result.Summary.ServiceCount = %d, want 1", result.Summary.ServiceCount)
	}
	if result.Summary.EnvironmentCount != 2 {
		t.Fatalf("result.Summary.EnvironmentCount = %d, want 2", result.Summary.EnvironmentCount)
	}
	if result.Summary.LintErrorCount != 2 {
		t.Fatalf("result.Summary.LintErrorCount = %d, want 2", result.Summary.LintErrorCount)
	}
	if result.Summary.LintWarningCount != 1 {
		t.Fatalf("result.Summary.LintWarningCount = %d, want 1", result.Summary.LintWarningCount)
	}
	if result.Summary.DriftIssueCount != 2 {
		t.Fatalf("result.Summary.DriftIssueCount = %d, want 2", result.Summary.DriftIssueCount)
	}
	if result.Summary.DriftEnvironmentCount != 2 {
		t.Fatalf("result.Summary.DriftEnvironmentCount = %d, want 2", result.Summary.DriftEnvironmentCount)
	}

	devRow := findStatusRow(t, result.Rows, "api", "dev")
	if devRow.Path != "env/api/dev.env" {
		t.Fatalf("devRow.Path = %q, want env/api/dev.env", devRow.Path)
	}
	if devRow.Lint.State != "error" {
		t.Fatalf("devRow.Lint.State = %q, want error", devRow.Lint.State)
	}
	if devRow.Lint.ErrorCount != 2 {
		t.Fatalf("devRow.Lint.ErrorCount = %d, want 2", devRow.Lint.ErrorCount)
	}
	if devRow.Lint.WarningCount != 1 {
		t.Fatalf("devRow.Lint.WarningCount = %d, want 1", devRow.Lint.WarningCount)
	}
	if devRow.Sync.State != "drift" {
		t.Fatalf("devRow.Sync.State = %q, want drift", devRow.Sync.State)
	}
	if devRow.Sync.IssueCount != 2 {
		t.Fatalf("devRow.Sync.IssueCount = %d, want 2", devRow.Sync.IssueCount)
	}
	if devRow.Sync.MissingCount != 1 {
		t.Fatalf("devRow.Sync.MissingCount = %d, want 1", devRow.Sync.MissingCount)
	}
	if devRow.Sync.PresentCount != 1 {
		t.Fatalf("devRow.Sync.PresentCount = %d, want 1", devRow.Sync.PresentCount)
	}
	if !devRow.UpdatedAt.Equal(devUpdatedAt) {
		t.Fatalf("devRow.UpdatedAt = %v, want %v", devRow.UpdatedAt, devUpdatedAt)
	}

	stgRow := findStatusRow(t, result.Rows, "api", "stg")
	if stgRow.Lint.State != "ok" {
		t.Fatalf("stgRow.Lint.State = %q, want ok", stgRow.Lint.State)
	}
	if stgRow.Sync.State != "drift" {
		t.Fatalf("stgRow.Sync.State = %q, want drift", stgRow.Sync.State)
	}
	if stgRow.Sync.IssueCount != 2 {
		t.Fatalf("stgRow.Sync.IssueCount = %d, want 2", stgRow.Sync.IssueCount)
	}
	if stgRow.Sync.MissingCount != 1 {
		t.Fatalf("stgRow.Sync.MissingCount = %d, want 1", stgRow.Sync.MissingCount)
	}
	if stgRow.Sync.PresentCount != 1 {
		t.Fatalf("stgRow.Sync.PresentCount = %d, want 1", stgRow.Sync.PresentCount)
	}
	if !stgRow.UpdatedAt.Equal(stgUpdatedAt) {
		t.Fatalf("stgRow.UpdatedAt = %v, want %v", stgRow.UpdatedAt, stgUpdatedAt)
	}
}

func TestStatus_WarningsDoNotMakeResultUnhealthy(t *testing.T) {
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
`,
		"env/api/dev.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
		"env/api/stg.env": "APP_ENV=stg\nEXTRA_FLAG=2\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Status(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.StatusOptions{})
	// Assert
	if err != nil {
		t.Fatalf("status env files: %v", err)
	}
	if !result.Healthy {
		t.Fatal("result.Healthy = false, want true")
	}
	if result.Summary.LintErrorCount != 0 {
		t.Fatalf("result.Summary.LintErrorCount = %d, want 0", result.Summary.LintErrorCount)
	}
	if result.Summary.LintWarningCount != 2 {
		t.Fatalf("result.Summary.LintWarningCount = %d, want 2", result.Summary.LintWarningCount)
	}
	if result.Summary.DriftIssueCount != 0 {
		t.Fatalf("result.Summary.DriftIssueCount = %d, want 0", result.Summary.DriftIssueCount)
	}
}

func findStatusRow(t *testing.T, rows []app.StatusRow, service, environment string) app.StatusRow {
	t.Helper()

	for _, row := range rows {
		if row.Service == service && row.Environment == environment {
			return row
		}
	}

	t.Fatalf("status row %s/%s: not found", service, environment)

	return app.StatusRow{}
}
