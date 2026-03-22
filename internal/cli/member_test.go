package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestRootCommand_MemberAddRekeysScopedFiles(t *testing.T) {
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
		"alice.pub":       "age1aliceexamplerecipient0000000000000000000000000000000\n",
	})

	var rekeyPaths []string
	adapter := &cryptotest.StubAdapter{
		RekeyFunc: func(_ context.Context, path string) error {
			rekeyPaths = append(rekeyPaths, path)
			return nil
		},
	}
	cmd := newRootCommandWithCryptoAdapter(t, adapter)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"member",
		"add",
		filepath.Join(root, "alice.pub"),
		"--scope", "api",
		"--rekey",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "updated .sops.yaml") {
		t.Fatalf("stdout = %q, want config update", stdout.String())
	}
	if !strings.Contains(stdout.String(), "rekeyed env/api/dev.env") || !strings.Contains(stdout.String(), "rekeyed env/api/stg.env") {
		t.Fatalf("stdout = %q, want rekey output", stdout.String())
	}
	if len(rekeyPaths) != 2 {
		t.Fatalf("len(adapter.rekeyPaths) = %d, want 2", len(rekeyPaths))
	}

	data, readErr := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if readErr != nil {
		t.Fatalf("read sops config: %v", readErr)
	}
	if !strings.Contains(string(data), "age1aliceexamplerecipient0000000000000000000000000000000") {
		t.Fatalf("sops config = %q, want recipient", string(data))
	}
}

func TestRootCommand_MemberRemoveUpdatesConfig(t *testing.T) {
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
    age:
      - age1aliceexamplerecipient0000000000000000000000000000000
`,
		"env/api/dev.env": "APP_ENV=dev\n",
	})

	cmd := newRootCommandWithCryptoAdapter(t, &cryptotest.StubAdapter{})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"member",
		"remove",
		"age1aliceexamplerecipient0000000000000000000000000000000",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "updated .sops.yaml") {
		t.Fatalf("stdout = %q, want config update", stdout.String())
	}

	data, readErr := os.ReadFile(filepath.Join(root, ".sops.yaml"))
	if readErr != nil {
		t.Fatalf("read sops config: %v", readErr)
	}
	if strings.Contains(string(data), "age1aliceexamplerecipient0000000000000000000000000000000") {
		t.Fatalf("sops config = %q, want recipient removed", string(data))
	}
}

func TestRootCommand_MemberAddJSON(t *testing.T) {
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
    age: []
`,
		"env/api/dev.env": "APP_ENV=dev\n",
		"alice.pub":       "age1aliceexamplerecipient0000000000000000000000000000000\n",
	})

	cmd := newRootCommandWithCryptoAdapter(t, &cryptotest.StubAdapter{})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--config", filepath.Join(root, "envdesk.yaml"),
		"member",
		"add",
		filepath.Join(root, "alice.pub"),
		"--json",
	})

	// Act
	err := cmd.Execute()
	// Assert
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	var result app.MemberResult
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("unmarshal json: %v", unmarshalErr)
	}
	if result.ConfigPath != filepath.Join(root, ".sops.yaml") {
		t.Fatalf("result.ConfigPath = %q, want .sops.yaml path", result.ConfigPath)
	}
}

func TestNewRootCommand_RegistersMember(t *testing.T) {
	// Arrange
	cmd := NewRootCommand()

	// Act
	found, _, err := cmd.Find([]string{"member"})
	// Assert
	if err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}
	if found == nil {
		t.Fatal("Find() command = nil, want member command")
	}
}
