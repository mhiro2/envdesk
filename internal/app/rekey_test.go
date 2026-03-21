package app_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRekey_SelectsMatchingTargets(t *testing.T) {
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
      stg: env/web/stg.env
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
		"env/web/dev.env": "APP_ENV=dev\n",
		"env/web/stg.env": "APP_ENV=stg\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	tests := []struct {
		name      string
		service   string
		env       string
		dryRun    bool
		wantFiles []string
		wantCalls []string
	}{
		{
			name:   "all files",
			dryRun: false,
			wantFiles: []string{
				filepath.Join(root, "env/api/dev.env"),
				filepath.Join(root, "env/api/stg.env"),
				filepath.Join(root, "env/web/dev.env"),
				filepath.Join(root, "env/web/stg.env"),
			},
			wantCalls: []string{
				filepath.Join(root, "env/api/dev.env"),
				filepath.Join(root, "env/api/stg.env"),
				filepath.Join(root, "env/web/dev.env"),
				filepath.Join(root, "env/web/stg.env"),
			},
		},
		{
			name:    "service filter",
			service: "api",
			dryRun:  false,
			wantFiles: []string{
				filepath.Join(root, "env/api/dev.env"),
				filepath.Join(root, "env/api/stg.env"),
			},
			wantCalls: []string{
				filepath.Join(root, "env/api/dev.env"),
				filepath.Join(root, "env/api/stg.env"),
			},
		},
		{
			name:   "dry run env filter",
			env:    "stg",
			dryRun: true,
			wantFiles: []string{
				filepath.Join(root, "env/api/stg.env"),
				filepath.Join(root, "env/web/stg.env"),
			},
			wantCalls: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var rekeyPaths []string
			adapter := &cryptotest.StubAdapter{
				RekeyFunc: func(_ context.Context, path string) error {
					rekeyPaths = append(rekeyPaths, path)
					return nil
				},
			}

			// Act
			result, err := app.Rekey(t.Context(), project, adapter, app.RekeyOptions{
				Service: tt.service,
				Env:     tt.env,
				DryRun:  tt.dryRun,
			})
			// Assert
			if err != nil {
				t.Fatalf("Rekey() error = %v, want nil", err)
			}
			if len(result.Files) != len(tt.wantFiles) {
				t.Fatalf("len(result.Files) = %d, want %d", len(result.Files), len(tt.wantFiles))
			}
			for idx := range tt.wantFiles {
				if result.Files[idx] != tt.wantFiles[idx] {
					t.Fatalf("result.Files[%d] = %q, want %q", idx, result.Files[idx], tt.wantFiles[idx])
				}
			}
			if len(rekeyPaths) != len(tt.wantCalls) {
				t.Fatalf("len(adapter.rekeyPaths) = %d, want %d", len(rekeyPaths), len(tt.wantCalls))
			}
			for idx := range tt.wantCalls {
				if rekeyPaths[idx] != tt.wantCalls[idx] {
					t.Fatalf("adapter.rekeyPaths[%d] = %q, want %q", idx, rekeyPaths[idx], tt.wantCalls[idx])
				}
			}
		})
	}
}

func TestRekey_RejectsMissingSelection(t *testing.T) {
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

	// Act
	_, err = app.Rekey(t.Context(), project, &cryptotest.StubAdapter{}, app.RekeyOptions{
		Env: "stg",
	})

	// Assert
	if err == nil {
		t.Fatal("Rekey() error = nil, want non-nil")
	}
	if err.Error() != `select env targets: no matching environment "stg"` {
		t.Fatalf("Rekey() error = %q, want missing selection", err.Error())
	}
}

func TestRekey_PartialFailure(t *testing.T) {
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

	callCount := 0
	failAdapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			callCount++
			if callCount >= 2 {
				return fmt.Errorf("rekey failed for %s", path)
			}

			return nil
		},
	}

	// Act
	_, err = app.Rekey(t.Context(), project, failAdapter, app.RekeyOptions{})

	// Assert
	if err == nil {
		t.Fatal("Rekey() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "rekey env file") {
		t.Fatalf("Rekey() error = %q, want rekey env file error", err.Error())
	}
}
