package app_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestDiff_ValueModePublic_HidesSecretValues(t *testing.T) {
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
		"env/api/dev.env": "APP_ENV=dev\nDATABASE_URL=https://dev.example.com\n",
		"env/api/stg.env": "APP_ENV=stg\nDATABASE_URL=https://stg.example.com\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.Diff(t.Context(), project, &cryptotest.PlaintextAdapter{}, "api", "dev", "stg", app.DiffOptions{
		ValueMode: app.DiffValueModePublic,
	})
	// Assert
	if err != nil {
		t.Fatalf("Diff() error = %v, want nil", err)
	}
	if result.Changes[0].Key != "APP_ENV" || result.Changes[0].From != "dev" || result.Changes[0].To != "stg" {
		t.Fatalf("result.Changes[0] = %#v, want public APP_ENV values", result.Changes[0])
	}
	if result.Changes[1].Key != "DATABASE_URL" || result.Changes[1].From != "(secret hidden)" || result.Changes[1].To != "(secret hidden)" {
		t.Fatalf("result.Changes[1] = %#v, want hidden secret values", result.Changes[1])
	}
}

func TestCheckSync_StrictRequiredOnly_FiltersOptionalAndUndeclared(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
      prod: env/api/prod.env
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg, prod]
    secret: false
  REQUIRED_TIMEOUT:
    required: true
    type: int
    secret: false
  OPTIONAL_FLAG:
    required: false
    type: bool
    secret: false
`,
		"env/api/dev.env":  "APP_ENV=dev\nREQUIRED_TIMEOUT=30\nOPTIONAL_FLAG=true\nEXTRA=1\n",
		"env/api/stg.env":  "APP_ENV=stg\n",
		"env/api/prod.env": "APP_ENV=prod\nREQUIRED_TIMEOUT=45\nEXTRA=1\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	issues, err := app.CheckSync(t.Context(), project, &cryptotest.PlaintextAdapter{}, app.CheckSyncOptions{
		StrictRequiredOnly: true,
	})
	// Assert
	if err != nil {
		t.Fatalf("CheckSync() error = %v, want nil", err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Key != "REQUIRED_TIMEOUT" || issues[0].Kind != app.SyncIssueKindRequired {
		t.Fatalf("issues[0] = %#v, want required drift only", issues[0])
	}
}

func TestSyncKeys_PlaceholdersFollowSchema(t *testing.T) {
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
    values: [dev, stg, prod]
    secret: false
  ENABLE_TLS:
    required: true
    type: bool
    secret: false
  PORT:
    required: true
    type: int
    secret: false
  API_TOKEN:
    required: true
    type: string
    secret: true
`,
		"env/api/dev.env": "APP_ENV=dev\nENABLE_TLS=true\nPORT=8443\nAPI_TOKEN=secret\n",
		"env/api/stg.env": "APP_ENV=stg\n",
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
		Placeholders:       true,
	})
	// Assert
	if err != nil {
		t.Fatalf("SyncKeys() error = %v, want nil", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, "env/api/stg.env"))
	if readErr != nil {
		t.Fatalf("read synced file: %v", readErr)
	}
	if string(data) != "APP_ENV=stg\nENABLE_TLS=false\nPORT=0\nAPI_TOKEN=\n" {
		t.Fatalf("synced file = %q, want schema-aware placeholders", string(data))
	}
}

func TestInit_EncryptMode_WritesEncryptedEnvFiles(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")
	adapter := &cryptotest.FakeEncryptAdapter{}

	// Act
	result, err := app.Init(t.Context(), adapter, app.InitOptions{
		ConfigPath:    configPath,
		ScaffoldSOPS:  true,
		Encrypt:       true,
		AgeRecipients: []string{"age1example"},
	})
	// Assert
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	if len(result.Files) != 6 {
		t.Fatalf("len(result.Files) = %d, want 6", len(result.Files))
	}
	sopsData, readErr := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if readErr != nil {
		t.Fatalf("read sops file: %v", readErr)
	}
	if !strings.Contains(string(sopsData), "age1example") {
		t.Fatalf("sops file = %q, want recipient", string(sopsData))
	}
	encrypted, readErr := os.ReadFile(filepath.Join(root, "env/api/dev.env"))
	if readErr != nil {
		t.Fatalf("read env file: %v", readErr)
	}
	decrypted, decryptErr := adapter.Decrypt(t.Context(), filepath.Join(root, "env/api/dev.env"))
	if decryptErr != nil {
		t.Fatalf("decrypt env file: %v", decryptErr)
	}
	if string(encrypted) == string(decrypted) {
		t.Fatal("env file should be encrypted on disk")
	}
	if string(decrypted) != "APP_ENV=dev\n" {
		t.Fatalf("decrypted env file = %q, want scaffolded plaintext", string(decrypted))
	}
}

func hasAncestorFile(targetPath, baseName string) bool {
	dir := filepath.Dir(filepath.Clean(targetPath))
	for {
		_, err := os.Stat(filepath.Join(dir, baseName))
		if err == nil {
			return true
		}
		if !os.IsNotExist(err) {
			return false
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// encryptAdapterRequiringSOPS fails Encrypt like SOPS when .sops.yaml is missing along the
// ancestor walk from the env file, so Init must write .sops.yaml before encrypting env files.
type encryptAdapterRequiringSOPS struct {
	cryptotest.FakeEncryptAdapter
}

func (a *encryptAdapterRequiringSOPS) Encrypt(ctx context.Context, path string, plaintext []byte) ([]byte, error) {
	if !hasAncestorFile(path, ".sops.yaml") {
		return nil, fmt.Errorf("lookup sops config for %q: not found", path)
	}

	out, err := a.FakeEncryptAdapter.Encrypt(ctx, path, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt scaffold file: %w", err)
	}

	return out, nil
}

func TestInit_EncryptMode_SOPSScaffoldPrecedesEnvEncryption(t *testing.T) {
	// Arrange
	root := t.TempDir()
	configPath := filepath.Join(root, "envdesk.yaml")

	// Act
	_, err := app.Init(t.Context(), &encryptAdapterRequiringSOPS{}, app.InitOptions{
		ConfigPath:    configPath,
		ScaffoldSOPS:  true,
		Encrypt:       true,
		AgeRecipients: []string{"age1example"},
	})
	// Assert
	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
}

func TestMemberAdd_DryRun_PreviewsTargetsWithoutWriting(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/api/.*\.env$
    age: []
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"env/api/stg.env": "APP_ENV=stg\n",
		"alice.pub":       "age1aliceexample\n",
	})

	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	// Act
	result, err := app.MemberAdd(t.Context(), project, &cryptotest.StubAdapter{
		RekeyFunc: func(context.Context, string) error {
			t.Fatal("Rekey() should not be called during dry-run")
			return nil
		},
	}, app.MemberOptions{
		Recipient: filepath.Join(root, "alice.pub"),
		Scope:     "api",
		DryRun:    true,
	})
	// Assert
	if err != nil {
		t.Fatalf("MemberAdd() error = %v, want nil", err)
	}
	if !result.DryRun {
		t.Fatal("result.DryRun = false, want true")
	}
	if len(result.AffectedFiles) != 2 {
		t.Fatalf("len(result.AffectedFiles) = %d, want 2", len(result.AffectedFiles))
	}
	data, readErr := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if readErr != nil {
		t.Fatalf("read sops config: %v", readErr)
	}
	if strings.Contains(string(data), "age1aliceexample") {
		t.Fatalf("sops config = %q, want unchanged file during dry-run", string(data))
	}
}
