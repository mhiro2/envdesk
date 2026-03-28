package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

type stubAuditRunner struct {
	blameOutputs map[string][]byte
	logRecords   []gitCommitRecord
	snapshots    map[string]map[string][]byte
	blameErr     error
	logErr       error
}

func (s *stubAuditRunner) Blame(_ context.Context, _, filePath string) ([]byte, error) {
	if s.blameErr != nil {
		return nil, s.blameErr
	}

	out, ok := s.blameOutputs[filePath]
	if !ok {
		return nil, fmt.Errorf("no blame output for %q", filePath)
	}

	return out, nil
}

func (s *stubAuditRunner) Log(_ context.Context, _ string, _ []string) ([]gitCommitRecord, error) {
	if s.logErr != nil {
		return nil, s.logErr
	}

	return s.logRecords, nil
}

func (s *stubAuditRunner) Show(_ context.Context, _, rev, filePath string) ([]byte, bool, error) {
	files, ok := s.snapshots[rev]
	if !ok {
		return nil, false, nil
	}

	data, ok := files[filePath]
	if !ok {
		return nil, false, nil
	}

	return data, true, nil
}

const sampleAuditBlameDev = `a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0 1 1 1
author Alice
author-time 1700000000
summary feat: add initial env
filename env/api/dev.env
	APP_ENV=dev
a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0 2 2 1
author Alice
author-time 1700000000
summary feat: add initial env
filename env/api/dev.env
	DATABASE_URL=https://dev.example.com
c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2 3 3 1
author Carol
author-time 1700200000
summary feat: add feature flag audit
filename env/api/dev.env
	FEATURE_FLAG=true
`

const sampleAuditBlameStg = `a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0 1 1 1
author Alice
author-time 1700000000
summary feat: add initial env
filename env/api/stg.env
	APP_ENV=stg
a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0 2 2 1
author Alice
author-time 1700000000
summary feat: add initial env
filename env/api/stg.env
	DATABASE_URL=https://stg.example.com
`

func TestAudit_TracksSchemaHistoryAndDriftSince(t *testing.T) {
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
  FEATURE_FLAG:
    required: false
    type: bool
    secret: false
`,
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=https://dev.example.com\nFEATURE_FLAG=true\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubAuditRunner{
		blameOutputs: map[string][]byte{
			"env/api/dev.env": []byte(sampleAuditBlameDev),
			"env/api/stg.env": []byte(sampleAuditBlameStg),
		},
		logRecords: []gitCommitRecord{
			{
				Fact: AuditFact{
					Author:    "Alice",
					Date:      time.Unix(1700000000, 0),
					CommitSHA: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
					Summary:   "feat: add initial env",
				},
				Paths: []string{"env/api/dev.env", "env/api/stg.env", "env.schema/api.yaml"},
			},
			{
				Fact: AuditFact{
					Author:    "Bob",
					Date:      time.Unix(1700100000, 0),
					CommitSHA: "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1",
					Summary:   "feat: tighten database schema",
				},
				Paths: []string{"env.schema/api.yaml"},
			},
			{
				Fact: AuditFact{
					Author:    "Carol",
					Date:      time.Unix(1700200000, 0),
					CommitSHA: "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2",
					Summary:   "feat: add feature flag audit",
				},
				Paths: []string{"env/api/dev.env", "env.schema/api.yaml"},
			},
		},
		snapshots: map[string]map[string][]byte{
			"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0": {
				"env/api/dev.env": []byte("APP_ENV=dev\nDATABASE_URL=https://dev.example.com\n"),
				"env/api/stg.env": []byte("APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n"),
				"env.schema/api.yaml": []byte(`keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
  DATABASE_URL:
    required: false
    type: string
    secret: false
`),
			},
			"b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1": {
				"env/api/dev.env": []byte("APP_ENV=dev\nDATABASE_URL=https://dev.example.com\n"),
				"env/api/stg.env": []byte("APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n"),
				"env.schema/api.yaml": []byte(`keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
`),
			},
			"c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2": {
				"env/api/dev.env": []byte("APP_ENV=dev\nDATABASE_URL=https://dev.example.com\nFEATURE_FLAG=true\n"),
				"env/api/stg.env": []byte("APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n"),
				"env.schema/api.yaml": []byte(`keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
  FEATURE_FLAG:
    required: false
    type: bool
    secret: false
`),
			},
		},
	}

	// Act
	results, err := auditWith(t.Context(), project, AuditOptions{Environment: "dev"}, runner)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Assert
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.Service != "api" {
		t.Fatalf("result.Service = %q, want %q", result.Service, "api")
	}
	if result.Environment != "dev" {
		t.Fatalf("result.Environment = %q, want %q", result.Environment, "dev")
	}
	if len(result.Entries) != 3 {
		t.Fatalf("len(result.Entries) = %d, want 3", len(result.Entries))
	}

	databaseURL := findAuditEntry(t, result.Entries, "DATABASE_URL")
	if databaseURL.LastValueChange == nil {
		t.Fatal("DATABASE_URL.LastValueChange = nil, want value")
	}
	if databaseURL.LastValueChange.Author != "Alice" {
		t.Fatalf("DATABASE_URL.LastValueChange.Author = %q, want %q", databaseURL.LastValueChange.Author, "Alice")
	}
	if databaseURL.Schema == nil {
		t.Fatal("DATABASE_URL.Schema = nil, want value")
	}
	if !databaseURL.Schema.Required {
		t.Fatal("DATABASE_URL.Schema.Required = false, want true")
	}
	if !databaseURL.Schema.Secret {
		t.Fatal("DATABASE_URL.Schema.Secret = false, want true")
	}
	if databaseURL.Schema.Type != "url" {
		t.Fatalf("DATABASE_URL.Schema.Type = %q, want %q", databaseURL.Schema.Type, "url")
	}
	if len(databaseURL.SchemaChanges) != 6 {
		t.Fatalf("len(DATABASE_URL.SchemaChanges) = %d, want 6", len(databaseURL.SchemaChanges))
	}
	if databaseURL.LastSchemaChange == nil {
		t.Fatal("DATABASE_URL.LastSchemaChange = nil, want value")
	}
	if databaseURL.LastSchemaChange.Author != "Bob" {
		t.Fatalf("DATABASE_URL.LastSchemaChange.Author = %q, want %q", databaseURL.LastSchemaChange.Author, "Bob")
	}

	featureFlag := findAuditEntry(t, result.Entries, "FEATURE_FLAG")
	if !featureFlag.Present {
		t.Fatal("FEATURE_FLAG.Present = false, want true")
	}
	if featureFlag.Drift.State != "drift" {
		t.Fatalf("FEATURE_FLAG.Drift.State = %q, want %q", featureFlag.Drift.State, "drift")
	}
	if featureFlag.Drift.Kind != SyncIssueKindOptional {
		t.Fatalf("FEATURE_FLAG.Drift.Kind = %q, want %q", featureFlag.Drift.Kind, SyncIssueKindOptional)
	}
	if featureFlag.Drift.Since == nil {
		t.Fatal("FEATURE_FLAG.Drift.Since = nil, want value")
	}
	if featureFlag.Drift.Since.Author != "Carol" {
		t.Fatalf("FEATURE_FLAG.Drift.Since.Author = %q, want %q", featureFlag.Drift.Since.Author, "Carol")
	}
	if !featureFlag.Drift.Since.Date.Equal(time.Unix(1700200000, 0)) {
		t.Fatalf("FEATURE_FLAG.Drift.Since.Date = %v, want %v", featureFlag.Drift.Since.Date, time.Unix(1700200000, 0))
	}
	if len(featureFlag.Events) == 0 {
		t.Fatal("len(FEATURE_FLAG.Events) = 0, want non-zero")
	}
}

func TestAudit_RejectsUnknownEnvironment(t *testing.T) {
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

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubAuditRunner{}

	// Act & Assert
	_, err = auditWith(t.Context(), project, AuditOptions{Environment: "prod"}, runner)
	if err == nil {
		t.Fatal("audit error = nil, want non-nil")
	}
	if err.Error() != `select environment "prod": not configured` {
		t.Fatalf("audit error = %q, want unknown environment", err.Error())
	}
}

func TestExtractEnvKey(t *testing.T) {
	// Arrange
	tests := []struct {
		line string
		want string
	}{
		{line: "APP_ENV=dev", want: "APP_ENV"},
		{line: "export DB_HOST=localhost", want: "DB_HOST"},
		{line: "# comment", want: ""},
		{line: "", want: ""},
		{line: "INVALID", want: ""},
		{line: "_PRIVATE=1", want: "_PRIVATE"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			// Act
			got := extractEnvKey(tt.line)

			// Assert
			if got != tt.want {
				t.Fatalf("extractEnvKey(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseGitLogRecords_Simple(t *testing.T) {
	// Arrange
	input := []byte("commit\x1fa1b2\x1fAlice\x1f1700000000\x1ffeat: add env\nenv/api/dev.env\nenv.schema/api.yaml\n")

	// Act
	records, err := parseGitLogRecords(input)
	if err != nil {
		t.Fatalf("parseGitLogRecords: %v", err)
	}

	// Assert
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Fact.Author != "Alice" {
		t.Fatalf("records[0].Fact.Author = %q, want %q", records[0].Fact.Author, "Alice")
	}
	if len(records[0].Paths) != 2 {
		t.Fatalf("len(records[0].Paths) = %d, want 2", len(records[0].Paths))
	}
}

func TestParsePorcelainBlame_Simple(t *testing.T) {
	// Act
	lines, commits, err := parsePorcelainBlame([]byte(sampleAuditBlameDev))
	if err != nil {
		t.Fatalf("parsePorcelainBlame: %v", err)
	}

	// Assert
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(lines))
	}
	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}
}

func findAuditEntry(t *testing.T, entries []AuditEntry, key string) AuditEntry {
	t.Helper()

	for _, entry := range entries {
		if entry.Key == key {
			return entry
		}
	}

	t.Fatalf("audit entry %q: not found", key)
	return AuditEntry{}
}
