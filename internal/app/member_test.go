package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestMemberAdd_UpdatesScopedRulesAndRekeys(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
  - name: web
    files:
      dev: env/web/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: ""
  - path_regex: ^env/web/.*\.env$
    age: age1webexistingrecipient0000000000000000000000000000000000
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
		"env/web/dev.env": "APP_ENV=dev\n",
		"alice.pub":       "age1aliceexamplerecipient0000000000000000000000000000000\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	var rekeyPaths []string
	adapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			rekeyPaths = append(rekeyPaths, path)
			return nil
		},
	}

	// Act
	result, err := app.MemberAdd(t.Context(), project, adapter, app.MemberOptions{
		Recipient: filepath.Join(root, "alice.pub"),
		Scope:     "api",
		Rekey:     true,
	})
	// Assert
	if err != nil {
		t.Fatalf("MemberAdd() error = %v, want nil", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(result.ConfigPath), ".sops.yaml") {
		t.Fatalf("result.ConfigPath = %q, want .sops.yaml", result.ConfigPath)
	}
	if len(result.RekeyedFiles) != 2 {
		t.Fatalf("len(result.RekeyedFiles) = %d, want 2", len(result.RekeyedFiles))
	}
	if len(rekeyPaths) != 2 {
		t.Fatalf("len(adapter.rekeyPaths) = %d, want 2", len(rekeyPaths))
	}
	if !strings.HasSuffix(filepath.ToSlash(rekeyPaths[0]), "env/api/dev.env") || !strings.HasSuffix(filepath.ToSlash(rekeyPaths[1]), "env/api/stg.env") {
		t.Fatalf("adapter.rekeyPaths = %#v, want api env files", rekeyPaths)
	}

	loaded := loadTestSOPSConfig(t, filepath.Join(root, ".sops.yaml"))
	if len(loaded.CreationRules) != 2 {
		t.Fatalf("len(loaded.CreationRules) = %d, want 2", len(loaded.CreationRules))
	}
	if loaded.CreationRules[0].Age != "age1aliceexamplerecipient0000000000000000000000000000000" {
		t.Fatalf("api age = %q, want added recipient", loaded.CreationRules[0].Age)
	}
	if loaded.CreationRules[1].Age != "age1webexistingrecipient0000000000000000000000000000000000" {
		t.Fatalf("web age = %q, want preserved recipient", loaded.CreationRules[1].Age)
	}
}

func TestMemberRemove_UpdatesRecipientsWithoutRekey(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: age1aliceexamplerecipient0000000000000000000000000000000, age1bobexamplerecipient000000000000000000000000000000000
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	adapter := &cryptotest.StubAdapter{}

	// Act
	result, err := app.MemberRemove(context.Background(), project, adapter, app.MemberOptions{
		Recipient: "age1aliceexamplerecipient0000000000000000000000000000000",
	})
	// Assert
	if err != nil {
		t.Fatalf("MemberRemove() error = %v, want nil", err)
	}
	if len(result.RekeyedFiles) != 0 {
		t.Fatalf("len(result.RekeyedFiles) = %d, want 0", len(result.RekeyedFiles))
	}

	loaded := loadTestSOPSConfig(t, filepath.Join(root, ".sops.yaml"))
	if len(loaded.CreationRules) != 1 {
		t.Fatalf("len(loaded.CreationRules) = %d, want 1", len(loaded.CreationRules))
	}
	if loaded.CreationRules[0].Age != "age1bobexamplerecipient000000000000000000000000000000000" {
		t.Fatalf("age = %q, want bob only", loaded.CreationRules[0].Age)
	}
}

func TestMemberRecipientValidation(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: age1aliceexamplerecipient0000000000000000000000000000000
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr string
	}{
		{
			name: "duplicate add",
			run: func() error {
				_, err := app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
					Recipient: "age1aliceexamplerecipient0000000000000000000000000000000",
				})
				if err != nil {
					return fmt.Errorf("add member recipient: %w", err)
				}

				return nil
			},
			wantErr: `add recipient "age1aliceexamplerecipient0000000000000000000000000000000": already configured`,
		},
		{
			name: "missing remove",
			run: func() error {
				_, err := app.MemberRemove(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
					Recipient: "age1bobexamplerecipient000000000000000000000000000000000",
				})
				if err != nil {
					return fmt.Errorf("remove member recipient: %w", err)
				}

				return nil
			},
			wantErr: `remove recipient "age1bobexamplerecipient000000000000000000000000000000000": not configured`,
		},
		{
			name: "empty recipient",
			run: func() error {
				_, err := app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
					Recipient: "   ",
				})
				if err != nil {
					return fmt.Errorf("add member recipient: %w", err)
				}

				return nil
			},
			wantErr: `resolve recipient: empty recipient`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.run()

			// Assert
			if err == nil {
				t.Fatal("error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func loadTestSOPSConfig(t *testing.T, path string) testSOPSConfig {
	t.Helper()

	// #nosec G304 -- the test only reads files created in a temp directory.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sops config: %v", err)
	}

	var loaded testSOPSConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse sops config: %v", err)
	}

	return loaded
}

type testSOPSConfig struct {
	CreationRules []struct {
		PathRegex string `yaml:"path_regex"`
		Age       string `yaml:"age"`
	} `yaml:"creation_rules"`
}

func TestMemberAdd_RecipientFromRelativeFile(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: ""
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"keys/alice.pub":  "age1aliceexamplerecipient0000000000000000000000000000000\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
		Recipient: "keys/alice.pub",
	})
	// Assert
	if err != nil {
		t.Fatalf("MemberAdd() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("MemberAdd() result = nil, want non-nil")
	}
}

func TestMemberAdd_RecipientDirectoryError(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: ""
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	// Create a directory where a file is expected
	keysDir := filepath.Join(root, "keys")
	if err := os.MkdirAll(keysDir, 0o750); err != nil {
		t.Fatalf("mkdir keys: %v", err)
	}

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	_, err = app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
		Recipient: keysDir,
	})

	// Assert
	if err == nil {
		t.Fatal("MemberAdd() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("MemberAdd() error = %q, want directory error", err.Error())
	}
}

func TestMemberAdd_EmptyRecipientFile(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: ""
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"empty.pub":       "\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	_, err = app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{}, app.MemberOptions{
		Recipient: filepath.Join(root, "empty.pub"),
	})

	// Assert
	if err == nil {
		t.Fatal("MemberAdd() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "empty recipient") {
		t.Fatalf("MemberAdd() error = %q, want empty recipient error", err.Error())
	}
}
