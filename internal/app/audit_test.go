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

type stubBlameRunner struct {
	output map[string][]byte
	err    error
}

func (s *stubBlameRunner) Run(_ context.Context, _, filePath string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	out, ok := s.output[filePath]
	if !ok {
		return nil, fmt.Errorf("no stub output for %q", filePath)
	}
	return out, nil
}

// porcelain blame output for a file with two keys: APP_ENV=dev and DB_HOST=localhost
const sampleBlameOutput = `a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0 1 1 1
author Alice
author-mail <alice@example.com>
author-time 1700000000
author-tz +0000
committer Alice
committer-mail <alice@example.com>
committer-time 1700000000
committer-tz +0000
summary feat: add initial env
filename env/api/dev.env
	APP_ENV=dev
b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1 2 2 1
author Bob
author-mail <bob@example.com>
author-time 1700100000
author-tz +0000
committer Bob
committer-mail <bob@example.com>
committer-time 1700100000
committer-tz +0000
summary feat: add database config
filename env/api/dev.env
	DB_HOST=localhost
`

func TestAudit_ReturnsEntriesFromBlame(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDB_HOST=localhost\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubBlameRunner{
		output: map[string][]byte{
			"env/api/dev.env": []byte(sampleBlameOutput),
		},
	}

	// Act
	results, err := auditWith(t.Context(), project, AuditOptions{}, runner)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Assert
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.Service != "api" {
		t.Errorf("service = %q, want %q", result.Service, "api")
	}
	if result.Environment != "dev" {
		t.Errorf("environment = %q, want %q", result.Environment, "dev")
	}
	if len(result.Entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(result.Entries))
	}

	entry0 := result.Entries[0]
	if entry0.Key != "APP_ENV" {
		t.Errorf("entries[0].Key = %q, want %q", entry0.Key, "APP_ENV")
	}
	if entry0.Author != "Alice" {
		t.Errorf("entries[0].Author = %q, want %q", entry0.Author, "Alice")
	}
	if entry0.CommitSHA != "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0" {
		t.Errorf("entries[0].CommitSHA = %q, want %q", entry0.CommitSHA, "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0")
	}
	if entry0.Summary != "feat: add initial env" {
		t.Errorf("entries[0].Summary = %q, want %q", entry0.Summary, "feat: add initial env")
	}
	expectedDate := time.Unix(1700000000, 0)
	if !entry0.Date.Equal(expectedDate) {
		t.Errorf("entries[0].Date = %v, want %v", entry0.Date, expectedDate)
	}

	entry1 := result.Entries[1]
	if entry1.Key != "DB_HOST" {
		t.Errorf("entries[1].Key = %q, want %q", entry1.Key, "DB_HOST")
	}
	if entry1.Author != "Bob" {
		t.Errorf("entries[1].Author = %q, want %q", entry1.Author, "Bob")
	}
}

func TestAudit_FilterByService(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
  - name: web
    files:
      dev: env/web/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/web/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubBlameRunner{
		output: map[string][]byte{
			"env/api/dev.env": []byte(sampleBlameOutput),
		},
	}

	// Act
	results, err := auditWith(t.Context(), project, AuditOptions{Service: "api"}, runner)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Assert
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Service != "api" {
		t.Errorf("service = %q, want %q", results[0].Service, "api")
	}
}

func TestAudit_FilterByKey(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "APP_ENV=dev\nDB_HOST=localhost\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubBlameRunner{
		output: map[string][]byte{
			"env/api/dev.env": []byte(sampleBlameOutput),
		},
	}

	// Act
	results, err := auditWith(t.Context(), project, AuditOptions{Key: "DB_HOST"}, runner)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Assert
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if len(results[0].Entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(results[0].Entries))
	}
	if results[0].Entries[0].Key != "DB_HOST" {
		t.Errorf("key = %q, want %q", results[0].Entries[0].Key, "DB_HOST")
	}
}

func TestAudit_FilterByEnvironment(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	runner := &stubBlameRunner{
		output: map[string][]byte{
			"env/api/dev.env": []byte(sampleBlameOutput),
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
	if results[0].Environment != "dev" {
		t.Errorf("environment = %q, want %q", results[0].Environment, "dev")
	}
}

func TestExtractEnvKey(t *testing.T) {
	// Arrange
	tests := []struct {
		line string
		want string
	}{
		{"APP_ENV=dev", "APP_ENV"},
		{"export DB_HOST=localhost", "DB_HOST"},
		{"# comment", ""},
		{"", ""},
		{"INVALID", ""},
		{"_PRIVATE=1", "_PRIVATE"},
	}

	for _, tt := range tests {
		// Act & Assert
		got := extractEnvKey(tt.line)
		if got != tt.want {
			t.Errorf("extractEnvKey(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestParsePorcelainBlame(t *testing.T) {
	// Act
	lines, commits, err := parsePorcelainBlame([]byte(sampleBlameOutput))
	if err != nil {
		t.Fatalf("parsePorcelainBlame: %v", err)
	}

	// Assert
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	if lines[0].content != "APP_ENV=dev" {
		t.Errorf("lines[0].content = %q, want %q", lines[0].content, "APP_ENV=dev")
	}
	if lines[1].content != "DB_HOST=localhost" {
		t.Errorf("lines[1].content = %q, want %q", lines[1].content, "DB_HOST=localhost")
	}

	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}

	sha1 := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	c1, ok := commits[sha1]
	if !ok {
		t.Fatalf("commit %q not found", sha1)
	}
	if c1.author != "Alice" {
		t.Errorf("commit author = %q, want %q", c1.author, "Alice")
	}
	if c1.summary != "feat: add initial env" {
		t.Errorf("commit summary = %q, want %q", c1.summary, "feat: add initial env")
	}
}

func TestAudit_BlameRunnerError(t *testing.T) {
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

	runner := &stubBlameRunner{
		err: fmt.Errorf("git blame failed"),
	}

	// Act & Assert
	_, err = auditWith(t.Context(), project, AuditOptions{}, runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
