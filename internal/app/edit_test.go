package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

type fakeEditRunner struct {
	runErr error
	path   string
	edit   func(string) error
}

func (f *fakeEditRunner) Run(_ context.Context, _, path string) error {
	f.path = path
	if f.edit != nil {
		if err := f.edit(path); err != nil {
			return err
		}
	}
	if f.runErr != nil {
		return f.runErr
	}

	return nil
}

func TestEdit_Success(t *testing.T) {
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
    values: [dev]
    secret: false
  DATABASE_URL:
    required: true
    type: url
    secret: true
`,
		"env/api/dev.env": "encrypted\n",
	})
	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	var encryptPath string
	var encryptIn []byte
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return []byte("APP_ENV=dev\nDATABASE_URL=https://before.example.com\n"), nil
		},
		EncryptFunc: func(_ context.Context, path string, plaintext []byte) ([]byte, error) {
			encryptPath = path
			encryptIn = append([]byte(nil), plaintext...)
			return []byte("ciphertext"), nil
		},
	}
	runner := &fakeEditRunner{
		edit: func(path string) error {
			return os.WriteFile(path, []byte("APP_ENV=dev\nDATABASE_URL=https://after.example.com\n"), 0o600)
		},
	}
	tempDir := t.TempDir()

	// Act
	result, err := editWithRunner(t.Context(), project, adapter, runner, EditOptions{
		Service:     "api",
		Environment: "dev",
		Editor:      "fake-editor",
		TempDir:     tempDir,
	})
	// Assert
	if err != nil {
		t.Fatalf("editWithRunner() error = %v, want nil", err)
	}
	if result.Path != filepath.Join(root, "env/api/dev.env") {
		t.Fatalf("result.Path = %q, want target path", result.Path)
	}
	if encryptPath != filepath.Join(root, "env/api/dev.env") {
		t.Fatalf("Encrypt() path = %q, want target env file", encryptPath)
	}
	if string(encryptIn) != "APP_ENV=dev\nDATABASE_URL=https://after.example.com\n" {
		t.Fatalf("Encrypt() input = %q, want edited plaintext", string(encryptIn))
	}

	written, readErr := os.ReadFile(filepath.Join(root, "env/api/dev.env"))
	if readErr != nil {
		t.Fatalf("read encrypted file: %v", readErr)
	}
	if string(written) != "ciphertext" {
		t.Fatalf("encrypted file = %q, want ciphertext", string(written))
	}

	entries, readDirErr := os.ReadDir(tempDir)
	if readDirErr != nil {
		t.Fatalf("ReadDir() error = %v", readDirErr)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir entries = %d, want 0", len(entries))
	}
}

func TestEdit_InvalidEdits(t *testing.T) {
	tests := []struct {
		name     string
		edit     string
		skipLint bool
		wantErr  string
	}{
		{
			name:    "parse error",
			edit:    "APP_ENV\n",
			wantErr: "parse edited env file for api/dev: line 1: parse assignment: missing '='",
		},
		{
			name:    "lint failure",
			edit:    "APP_ENV=local\n",
			wantErr: "validate edited env file for api/dev: error APP_ENV: invalid value: validate enum: expected one of [dev]",
		},
		{
			name:     "lint skipped",
			edit:     "APP_ENV=local\n",
			skipLint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
    values: [dev]
    secret: false
`,
				"env/api/dev.env": "encrypted\n",
			})
			project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
			if err != nil {
				t.Fatalf("load project: %v", err)
			}

			var encryptPath string
			var encryptIn []byte
			adapter := &cryptotest.StubAdapter{
				DecryptFunc: func(context.Context, string) ([]byte, error) {
					return []byte("APP_ENV=dev\n"), nil
				},
				EncryptFunc: func(_ context.Context, path string, plaintext []byte) ([]byte, error) {
					encryptPath = path
					encryptIn = append([]byte(nil), plaintext...)
					return []byte("ciphertext"), nil
				},
			}
			runner := &fakeEditRunner{
				edit: func(path string) error {
					return os.WriteFile(path, []byte(tt.edit), 0o600)
				},
			}
			tempDir := t.TempDir()

			// Act
			_, err = editWithRunner(t.Context(), project, adapter, runner, EditOptions{
				Service:     "api",
				Environment: "dev",
				Editor:      "fake-editor",
				SkipLint:    tt.skipLint,
				TempDir:     tempDir,
			})

			// Assert
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("editWithRunner() error = %v, want nil", err)
				}
				if string(encryptIn) != tt.edit {
					t.Fatalf("Encrypt() input = %q, want %q", string(encryptIn), tt.edit)
				}
			} else {
				if err == nil {
					t.Fatal("editWithRunner() error = nil, want non-nil")
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("editWithRunner() error = %q, want %q", err.Error(), tt.wantErr)
				}
				if encryptPath != "" {
					t.Fatalf("Encrypt() path = %q, want empty", encryptPath)
				}
			}

			written, readErr := os.ReadFile(filepath.Join(root, "env/api/dev.env"))
			if readErr != nil {
				t.Fatalf("read encrypted file: %v", readErr)
			}
			wantFile := "encrypted\n"
			if tt.wantErr == "" {
				wantFile = "ciphertext"
			}
			if string(written) != wantFile {
				t.Fatalf("encrypted file = %q, want %q", string(written), wantFile)
			}

			entries, readDirErr := os.ReadDir(tempDir)
			if readDirErr != nil {
				t.Fatalf("ReadDir() error = %v", readDirErr)
			}
			if len(entries) != 0 {
				t.Fatalf("temp dir entries = %d, want 0", len(entries))
			}
		})
	}
}

func TestEdit_PreservesSupportedFormatting(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "encrypted\n",
	})
	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	edited := []byte("# keep comment\nexport APP_ENV=dev\nDATABASE_URL='https://after.example.com'\n\nINLINE=value # trailing comment\n")
	var encryptIn []byte
	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return []byte("APP_ENV=dev\n"), nil
		},
		EncryptFunc: func(_ context.Context, _ string, plaintext []byte) ([]byte, error) {
			encryptIn = append([]byte(nil), plaintext...)
			return []byte("ciphertext"), nil
		},
	}
	runner := &fakeEditRunner{
		edit: func(path string) error {
			return os.WriteFile(path, edited, 0o600)
		},
	}

	// Act
	_, err = editWithRunner(t.Context(), project, adapter, runner, EditOptions{
		Service:     "api",
		Environment: "dev",
		Editor:      "fake-editor",
		TempDir:     t.TempDir(),
	})
	// Assert
	if err != nil {
		t.Fatalf("editWithRunner() error = %v, want nil", err)
	}
	if string(encryptIn) != string(edited) {
		t.Fatalf("Encrypt() input = %q, want %q", string(encryptIn), string(edited))
	}
}

func TestEdit_EditorFailureCleansUpTempFile(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
		"env/api/dev.env": "encrypted\n",
	})
	project, err := config.Load(filepath.Join(root, "envdesk.yaml"))
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	adapter := &cryptotest.StubAdapter{
		DecryptFunc: func(context.Context, string) ([]byte, error) {
			return []byte("APP_ENV=dev\n"), nil
		},
	}
	runner := &fakeEditRunner{
		runErr: fmt.Errorf("run editor %q: exit code 23: exit status 23", "fake-editor"),
	}
	tempDir := t.TempDir()

	// Act
	_, err = editWithRunner(t.Context(), project, adapter, runner, EditOptions{
		Service:     "api",
		Environment: "dev",
		Editor:      "fake-editor",
		TempDir:     tempDir,
	})

	// Assert
	if err == nil {
		t.Fatal("editWithRunner() error = nil, want non-nil")
	}
	if err.Error() != `edit env file for api/dev: run editor "fake-editor": exit code 23: exit status 23` {
		t.Fatalf("editWithRunner() error = %q, want editor exit code", err.Error())
	}

	entries, readDirErr := os.ReadDir(tempDir)
	if readDirErr != nil {
		t.Fatalf("ReadDir() error = %v", readDirErr)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir entries = %d, want 0", len(entries))
	}
}

func TestResolveEditor(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		envValue   string
		visual     string
		wantEditor string
		wantErr    string
	}{
		{
			name:       "flag value",
			flagValue:  "vim",
			envValue:   "nano",
			wantEditor: "vim",
		},
		{
			name:       "editor env",
			envValue:   "nano",
			wantEditor: "nano",
		},
		{
			name:       "visual env",
			visual:     "code --wait",
			wantEditor: "code --wait",
		},
		{
			name:    "missing editor",
			wantErr: "resolve editor: empty editor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			t.Setenv("EDITOR", tt.envValue)
			t.Setenv("VISUAL", tt.visual)

			// Act
			editor, err := resolveEditor(tt.flagValue)

			// Assert
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("resolveEditor() error = %v, want nil", err)
				}
				if editor != tt.wantEditor {
					t.Fatalf("resolveEditor() = %q, want %q", editor, tt.wantEditor)
				}
				return
			}

			if err == nil {
				t.Fatal("resolveEditor() error = nil, want non-nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("resolveEditor() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBuildEditorCommand(t *testing.T) {
	tests := []struct {
		name        string
		shell       editorShell
		editor      string
		path        string
		wantName    string
		wantArgs    []string
		wantEnvLast string
		wantErr     string
	}{
		{
			name: "unix shell",
			shell: editorShell{
				Command:      "/bin/sh",
				Args:         []string{"-c"},
				PathArgument: `"$` + editPathEnvVar + `"`,
			},
			editor:      "code --wait",
			path:        "/tmp/dev.env",
			wantName:    "/bin/sh",
			wantArgs:    []string{"-c", `code --wait "$` + editPathEnvVar + `"`},
			wantEnvLast: editPathEnvVar + "=/tmp/dev.env",
		},
		{
			name: "windows shell",
			shell: editorShell{
				Command:      "cmd.exe",
				Args:         []string{"/d", "/s", "/c"},
				PathArgument: `"%` + editPathEnvVar + `%"`,
			},
			editor:      `code.cmd --wait`,
			path:        `C:\tmp\dev.env`,
			wantName:    "cmd.exe",
			wantArgs:    []string{"/d", "/s", "/c", `code.cmd --wait "%` + editPathEnvVar + `%"`},
			wantEnvLast: editPathEnvVar + `=C:\tmp\dev.env`,
		},
		{
			name:    "empty editor",
			shell:   editorShell{Command: "/bin/sh", Args: []string{"-c"}, PathArgument: `"$` + editPathEnvVar + `"`},
			editor:  "   ",
			path:    "/tmp/dev.env",
			wantErr: "run editor: empty editor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange

			// Act
			command, err := buildEditorCommand(t.Context(), tt.editor, tt.path, tt.shell)

			// Assert
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("buildEditorCommand() error = nil, want non-nil")
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("buildEditorCommand() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}
			if err != nil {
				t.Fatalf("buildEditorCommand() error = %v, want nil", err)
			}
			if command.Path != tt.wantName {
				t.Fatalf("command.Path = %q, want %q", command.Path, tt.wantName)
			}
			if !reflect.DeepEqual(command.Args, append([]string{tt.wantName}, tt.wantArgs...)) {
				t.Fatalf("command.Args = %#v, want %#v", command.Args, append([]string{tt.wantName}, tt.wantArgs...))
			}
			if len(command.Env) == 0 {
				t.Fatal("command.Env = nil, want edit path env")
			}
			if command.Env[len(command.Env)-1] != tt.wantEnvLast {
				t.Fatalf("last command env = %q, want %q", command.Env[len(command.Env)-1], tt.wantEnvLast)
			}
		})
	}
}

func TestFormatProblems_MultipleProblems(t *testing.T) {
	// Arrange
	problems := []Problem{
		{
			Severity: SeverityError,
			Key:      "APP_ENV",
			Message:  "missing required key",
		},
		{
			Severity: SeverityWarning,
			Key:      "EXTRA_FLAG",
			Message:  "key not declared in schema",
		},
	}

	// Act
	got := formatProblems(problems)

	// Assert
	want := "error APP_ENV: missing required key; warning EXTRA_FLAG: key not declared in schema"
	if got != want {
		t.Fatalf("formatProblems() = %q, want %q", got, want)
	}
}
