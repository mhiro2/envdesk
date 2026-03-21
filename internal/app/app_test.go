package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestDiff_ModifiedAddedRemoved(t *testing.T) {
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
  API_BASE_URL:
    required: true
    type: url
    secret: false
  EXTRA_FLAG:
    required: false
    type: string
    secret: false
`,
		"env/api/dev.env": "APP_ENV=dev\nAPI_BASE_URL=https://dev.example.com\nDEV_ONLY=1\n",
		"env/api/stg.env": "APP_ENV=stg\nAPI_BASE_URL=https://stg.example.com\nEXTRA_FLAG=enabled\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{ShowMetadata: true})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.Changes) != 4 {
		t.Fatalf("len(result.Changes) = %d, want 4", len(result.Changes))
	}
	if result.Changes[0].Type != "modify" || result.Changes[0].Key != "API_BASE_URL" {
		t.Fatalf("first change = %#v, want modify API_BASE_URL", result.Changes[0])
	}
	if result.Changes[1].Type != "modify" || result.Changes[1].Key != "APP_ENV" {
		t.Fatalf("second change = %#v, want modify APP_ENV", result.Changes[1])
	}
	if result.Changes[2].Type != "remove" || result.Changes[2].Key != "DEV_ONLY" {
		t.Fatalf("third change = %#v, want remove DEV_ONLY", result.Changes[2])
	}
	if result.Changes[3].Type != "add" || result.Changes[3].Key != "EXTRA_FLAG" {
		t.Fatalf("fourth change = %#v, want add EXTRA_FLAG", result.Changes[3])
	}
	if result.Changes[0].Metadata == nil || result.Changes[0].Metadata.Type != "url" {
		t.Fatalf("metadata = %#v, want type url", result.Changes[0].Metadata)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("len(result.Findings) = %d, want 1", len(result.Findings))
	}
	if result.Findings[0].Severity != app.SeverityWarning || result.Findings[0].Key != "DEV_ONLY" {
		t.Fatalf("result.Findings[0] = %#v, want DEV_ONLY warning", result.Findings[0])
	}
	if result.Summary.Total != 5 || result.Summary.Added != 1 || result.Summary.Removed != 1 || result.Summary.Modified != 2 || result.Summary.Violations != 1 {
		t.Fatalf("summary = %#v, want 1 added, 1 removed, 2 modified, 1 violation", result.Summary)
	}
}

func TestDiff_IgnoresKeyOrderChanges(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=postgres://dev\nFEATURE_FLAG=true\n",
		"env/api/stg.env": "FEATURE_FLAG=true\nDATABASE_URL=postgres://dev\nAPP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("len(result.Changes) = %d, want 0", len(result.Changes))
	}
	if result.Summary.Total != 0 {
		t.Fatalf("result.Summary.Total = %d, want 0", result.Summary.Total)
	}
}

func TestDiff_DetectsRenameCandidates(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nLEGACY_FLAG=shared\nKEEP=1\n",
		"env/api/stg.env": "APP_ENV=dev\nNEW_FLAG=shared\nKEEP=1\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.RenameCandidates) != 1 {
		t.Fatalf("len(result.RenameCandidates) = %d, want 1", len(result.RenameCandidates))
	}
	if result.RenameCandidates[0].From != "LEGACY_FLAG" || result.RenameCandidates[0].To != "NEW_FLAG" {
		t.Fatalf("rename candidate = %#v, want LEGACY_FLAG -> NEW_FLAG", result.RenameCandidates[0])
	}
	if result.Summary.Renamed != 1 {
		t.Fatalf("result.Summary.Renamed = %d, want 1", result.Summary.Renamed)
	}
	if result.Summary.Total != 2 {
		t.Fatalf("result.Summary.Total = %d, want 2 (renamed excluded from total)", result.Summary.Total)
	}
}

func TestDiff_ShowsSchemaFindings(t *testing.T) {
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
		"env/api/dev.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
		"env/api/stg.env": "APP_ENV=dev\nEXTRA_FLAG=1\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{ShowMetadata: true})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("len(result.Changes) = %d, want 0", len(result.Changes))
	}
	if len(result.Findings) != 4 {
		t.Fatalf("len(result.Findings) = %d, want 4", len(result.Findings))
	}
	if result.Summary.Violations != 4 {
		t.Fatalf("result.Summary.Violations = %d, want 4", result.Summary.Violations)
	}
	if result.Summary.Total != 4 {
		t.Fatalf("result.Summary.Total = %d, want 4", result.Summary.Total)
	}
}

func TestLint_SchemaViolations(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
		"env.schema/api.yaml": `keys:
  DATABASE_URL:
    required: true
    type: url
    secret: true
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg, prod]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=local\nEXTRA_FLAG=1\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Lint(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.LintOptions{})
	// Assert
	if err != nil {
		t.Fatalf("lint env files: %v", err)
	}
	if len(result.Problems) != 3 {
		t.Fatalf("len(result.Problems) = %d, want 3", len(result.Problems))
	}
	if result.ErrorCount != 2 {
		t.Fatalf("result.ErrorCount = %d, want 2", result.ErrorCount)
	}
	if result.WarningCount != 1 {
		t.Fatalf("result.WarningCount = %d, want 1", result.WarningCount)
	}
	if result.Problems[0].Severity != app.SeverityError || result.Problems[0].Key != "APP_ENV" {
		t.Fatalf("result.Problems[0] = %#v, want APP_ENV error", result.Problems[0])
	}
	if result.Problems[1].Severity != app.SeverityError || result.Problems[1].Key != "DATABASE_URL" {
		t.Fatalf("result.Problems[1] = %#v, want DATABASE_URL error", result.Problems[1])
	}
	if result.Problems[2].Severity != app.SeverityWarning || result.Problems[2].Key != "EXTRA_FLAG" {
		t.Fatalf("result.Problems[2] = %#v, want EXTRA_FLAG warning", result.Problems[2])
	}
}

func TestCheckSync_DetectsMissingKeys(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
`,
		"env/api/dev.env":  "APP_ENV=dev\nPAYMENT_TIMEOUT=30\n",
		"env/api/stg.env":  "APP_ENV=stg\n",
		"env/api/prod.env": "APP_ENV=prod\nPAYMENT_TIMEOUT=60\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.CheckSync(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.CheckSyncOptions{})
	// Assert
	if err != nil {
		t.Fatalf("check sync: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Key != "PAYMENT_TIMEOUT" {
		t.Fatalf("result[0].Key = %q, want PAYMENT_TIMEOUT", result[0].Key)
	}
	if len(result[0].Missing) != 1 || result[0].Missing[0] != "stg" {
		t.Fatalf("result[0].Missing = %#v, want [stg]", result[0].Missing)
	}
}

func TestSyncKeys_WritePlaceholders(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=\nFEATURE_FLAG=\n",
		"env/api/stg.env": "APP_ENV=stg\nLEGACY_FLAG=true\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.SyncKeys(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.SyncKeysOptions{
		Service:           "api",
		SourceEnvironment: "dev",
		TargetEnvironments: []string{
			"stg",
		},
		Placeholders: true,
	})
	// Assert
	if err != nil {
		t.Fatalf("sync keys: %v", err)
	}
	if len(result.TargetEnvironments) != 1 {
		t.Fatalf("len(result.TargetEnvironments) = %d, want 1", len(result.TargetEnvironments))
	}
	target := result.TargetEnvironments[0]
	if len(target.Added) != 2 || target.Added[0] != "DATABASE_URL" || target.Added[1] != "FEATURE_FLAG" {
		t.Fatalf("target.Added = %#v, want DATABASE_URL and FEATURE_FLAG", target.Added)
	}
	if len(target.Removed) != 1 || target.Removed[0] != "LEGACY_FLAG" {
		t.Fatalf("target.Removed = %#v, want LEGACY_FLAG", target.Removed)
	}

	docPath := filepath.Join(root, "env/api/stg.env")
	updated, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read synced file: %v", err)
	}
	if string(updated) != "APP_ENV=stg\nDATABASE_URL=\nFEATURE_FLAG=\n" {
		t.Fatalf("synced file = %q, want normalized placeholder output", string(updated))
	}
}

func TestSyncKeys_DryRunLeavesTargetUntouched(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=\nFEATURE_FLAG=\n",
		"env/api/stg.env": "APP_ENV=stg\nLEGACY_FLAG=true\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	_, err = app.SyncKeys(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.SyncKeysOptions{
		Service:            "api",
		SourceEnvironment:  "dev",
		TargetEnvironments: []string{"stg"},
		DryRun:             true,
	})
	// Assert
	if err != nil {
		t.Fatalf("sync keys: %v", err)
	}

	updated, readErr := os.ReadFile(filepath.Join(root, "env/api/stg.env"))
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(updated) != "APP_ENV=stg\nLEGACY_FLAG=true\n" {
		t.Fatalf("target file = %q, want unchanged content", string(updated))
	}
}

func TestSyncKeys_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		options app.SyncKeysOptions
		wantErr string
	}{
		{
			name: "duplicate targets",
			files: map[string]string{
				"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
				"env/api/dev.env": "APP_ENV=dev\n",
				"env/api/stg.env": "APP_ENV=stg\n",
			},
			options: app.SyncKeysOptions{
				Service:            "api",
				SourceEnvironment:  "dev",
				TargetEnvironments: []string{"stg", "stg"},
			},
			wantErr: "duplicate target",
		},
		{
			name: "source target match",
			files: map[string]string{
				"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
				"env/api/dev.env": "APP_ENV=dev\n",
				"env/api/stg.env": "APP_ENV=stg\n",
			},
			options: app.SyncKeysOptions{
				Service:            "api",
				SourceEnvironment:  "dev",
				TargetEnvironments: []string{"dev"},
			},
			wantErr: "matches source environment",
		},
		{
			name: "invalid target",
			files: map[string]string{
				"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
				"env/api/dev.env": "APP_ENV=dev\n",
			},
			options: app.SyncKeysOptions{
				Service:            "api",
				SourceEnvironment:  "dev",
				TargetEnvironments: []string{"stg"},
			},
			wantErr: "lookup target environment",
		},
		{
			name: "non-empty source value",
			files: map[string]string{
				"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
				"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=postgres://dev\n",
				"env/api/stg.env": "APP_ENV=stg\n",
			},
			options: app.SyncKeysOptions{
				Service:            "api",
				SourceEnvironment:  "dev",
				TargetEnvironments: []string{"stg"},
			},
			wantErr: "require --placeholders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := projecttest.WriteProject(t, tt.files)

			project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
			if err != nil {
				t.Fatalf("load project: %v", err)
			}

			// Act
			_, err = app.SyncKeys(t.Context(), project, &cryptotest.PlaintextAdapter{}, tt.options)

			// Assert
			if err == nil {
				t.Fatal("SyncKeys() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SyncKeys() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDiff_RenameCandidatesExcludedFromTotal(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nOLD_KEY=shared\n",
		"env/api/stg.env": "APP_ENV=stg\nNEW_KEY=shared\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if result.Summary.Renamed != 1 {
		t.Fatalf("result.Summary.Renamed = %d, want 1", result.Summary.Renamed)
	}
	// Total should count add + remove + modified only, not rename candidates
	if result.Summary.Total != 3 {
		t.Fatalf("result.Summary.Total = %d, want 3 (1 added + 1 removed + 1 modified, renamed excluded)", result.Summary.Total)
	}
}

func TestDiff_EmptyValuesSkippedForRenameCandidates(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nOLD_A=\nOLD_B=\n",
		"env/api/stg.env": "APP_ENV=dev\nNEW_A=\nNEW_B=\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.RenameCandidates) != 0 {
		t.Fatalf("len(result.RenameCandidates) = %d, want 0 (empty values should be skipped)", len(result.RenameCandidates))
	}
	if result.Summary.Renamed != 0 {
		t.Fatalf("result.Summary.Renamed = %d, want 0", result.Summary.Renamed)
	}
}

func TestDiff_NoChangesSummary(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
		"env/api/stg.env": "APP_ENV=dev\nFEATURE_FLAG=true\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("len(result.Changes) = %d, want 0", len(result.Changes))
	}
	if result.Summary.Total != 0 || result.Summary.Added != 0 || result.Summary.Removed != 0 || result.Summary.Modified != 0 {
		t.Fatalf("summary = %#v, want zero counts", result.Summary)
	}
}

// --- Encrypted env integration tests ---
// These tests exercise the full decrypt -> parse -> logic -> encrypt pipeline
// using FakeEncryptAdapter (base64-based) to verify that review commands
// work correctly with "encrypted" env files.

func TestLint_Encrypted_SchemaViolations(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg, prod]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
`,
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=local\nEXTRA_FLAG=1\n"),
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Lint(t.Context(), project, &cryptotest.FakeEncryptAdapter{}, app.LintOptions{})
	// Assert
	if err != nil {
		t.Fatalf("lint env files: %v", err)
	}
	if result.ErrorCount != 2 {
		t.Fatalf("result.ErrorCount = %d, want 2", result.ErrorCount)
	}
	if result.WarningCount != 1 {
		t.Fatalf("result.WarningCount = %d, want 1", result.WarningCount)
	}
}

func TestDiff_Encrypted_DetectsChanges(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nDATABASE_URL=postgres://dev\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\nFEATURE_FLAG=true\n"),
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.FakeEncryptAdapter{}, "api", "dev", "stg", app.DiffOptions{})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if result.Summary.Added != 1 {
		t.Fatalf("result.Summary.Added = %d, want 1 (FEATURE_FLAG)", result.Summary.Added)
	}
	if result.Summary.Removed != 1 {
		t.Fatalf("result.Summary.Removed = %d, want 1 (DATABASE_URL)", result.Summary.Removed)
	}
	if result.Summary.Modified != 1 {
		t.Fatalf("result.Summary.Modified = %d, want 1 (APP_ENV)", result.Summary.Modified)
	}
}

func TestCheckSync_Encrypted_DetectsMissingKeys(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
`,
		"env/api/dev.env":  cryptotest.FakeEncryptContent("APP_ENV=dev\nSECRET_KEY=abc\n"),
		"env/api/stg.env":  cryptotest.FakeEncryptContent("APP_ENV=stg\n"),
		"env/api/prod.env": cryptotest.FakeEncryptContent("APP_ENV=prod\nSECRET_KEY=xyz\n"),
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.CheckSync(t.Context(), project, &cryptotest.FakeEncryptAdapter{}, app.CheckSyncOptions{})
	// Assert
	if err != nil {
		t.Fatalf("check sync: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Key != "SECRET_KEY" {
		t.Fatalf("result[0].Key = %q, want SECRET_KEY", result[0].Key)
	}
	if len(result[0].Missing) != 1 || result[0].Missing[0] != "stg" {
		t.Fatalf("result[0].Missing = %#v, want [stg]", result[0].Missing)
	}
}

func TestSyncKeys_Encrypted_WritesEncryptedOutput(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nDATABASE_URL=\nFEATURE_FLAG=\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\nLEGACY_FLAG=true\n"),
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	adapter := &cryptotest.FakeEncryptAdapter{}

	// Act
	result, err := app.SyncKeys(t.Context(), project, adapter, app.SyncKeysOptions{
		Service:            "api",
		SourceEnvironment:  "dev",
		TargetEnvironments: []string{"stg"},
		Placeholders:       true,
	})
	// Assert
	if err != nil {
		t.Fatalf("sync keys: %v", err)
	}
	if len(result.TargetEnvironments) != 1 {
		t.Fatalf("len(result.TargetEnvironments) = %d, want 1", len(result.TargetEnvironments))
	}
	target := result.TargetEnvironments[0]
	if len(target.Added) != 2 {
		t.Fatalf("target.Added = %#v, want 2 added keys", target.Added)
	}

	// Verify the file on disk is "encrypted" (base64) and round-trips correctly
	docPath := filepath.Join(root, "env/api/stg.env")
	encrypted, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read synced file: %v", err)
	}
	decrypted, err := adapter.Decrypt(t.Context(), docPath)
	if err != nil {
		t.Fatalf("decrypt synced file: %v", err)
	}
	if string(encrypted) == string(decrypted) {
		t.Fatal("synced file should be encrypted on disk, but file content equals decrypted content")
	}
	if string(decrypted) != "APP_ENV=stg\nDATABASE_URL=\nFEATURE_FLAG=\n" {
		t.Fatalf("decrypted synced file = %q, want normalized placeholder output", string(decrypted))
	}
}

func TestDiff_Encrypted_WithSchemaFindings(t *testing.T) {
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
		"env/api/dev.env": cryptotest.FakeEncryptContent("APP_ENV=dev\nEXTRA_FLAG=1\n"),
		"env/api/stg.env": cryptotest.FakeEncryptContent("APP_ENV=stg\nDATABASE_URL=https://db.example.com\n"),
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.FakeEncryptAdapter{}, "api", "dev", "stg", app.DiffOptions{ShowMetadata: true})
	// Assert
	if err != nil {
		t.Fatalf("diff env files: %v", err)
	}
	if result.Summary.Added != 1 {
		t.Fatalf("result.Summary.Added = %d, want 1 (DATABASE_URL)", result.Summary.Added)
	}
	if result.Summary.Removed != 1 {
		t.Fatalf("result.Summary.Removed = %d, want 1 (EXTRA_FLAG)", result.Summary.Removed)
	}
	// Schema findings: dev missing DATABASE_URL (error) + dev has EXTRA_FLAG (warning)
	// stg is valid (APP_ENV=stg, DATABASE_URL present)
	if result.Summary.Violations < 2 {
		t.Fatalf("result.Summary.Violations = %d, want at least 2", result.Summary.Violations)
	}
}
