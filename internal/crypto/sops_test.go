package crypto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeRunner struct {
	lookPathErr error
	lookPathArg string
	runResult   *Result
	runErr      error
	runCommand  Command
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	f.lookPathArg = file
	if f.lookPathErr != nil {
		return "", f.lookPathErr
	}

	return "/usr/bin/sops", nil
}

func (f *fakeRunner) Run(_ context.Context, cmd Command) (*Result, error) {
	f.runCommand = cmd
	if f.runResult == nil {
		f.runResult = &Result{}
	}

	return f.runResult, f.runErr
}

func TestSOPS_Check(t *testing.T) {
	// Arrange
	runner := &fakeRunner{}
	adapter := NewSOPSWithRunner(runner)

	// Act
	err := adapter.Check(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if runner.lookPathArg != "sops" {
		t.Fatalf("LookPath() arg = %q, want sops", runner.lookPathArg)
	}
}

func TestSOPS_Decrypt(t *testing.T) {
	// Arrange
	runner := &fakeRunner{
		runResult: &Result{Stdout: []byte("APP_ENV=dev\n")},
	}
	adapter := NewSOPSWithRunner(runner)
	targetPath := filepath.Join(t.TempDir(), "env", "api", "dev.env")

	// Act
	plaintext, err := adapter.Decrypt(t.Context(), targetPath)
	// Assert
	if err != nil {
		t.Fatalf("Decrypt() error = %v, want nil", err)
	}
	if string(plaintext) != "APP_ENV=dev\n" {
		t.Fatalf("Decrypt() = %q, want plaintext", string(plaintext))
	}
	if runner.runCommand.Name != "sops" {
		t.Fatalf("command name = %q, want sops", runner.runCommand.Name)
	}
	if len(runner.runCommand.Args) != 2 || runner.runCommand.Args[0] != "decrypt" || runner.runCommand.Args[1] != targetPath {
		t.Fatalf("command args = %#v, want decrypt target path", runner.runCommand.Args)
	}
	if runner.runCommand.Dir != filepath.Dir(targetPath) {
		t.Fatalf("command dir = %q, want %q", runner.runCommand.Dir, filepath.Dir(targetPath))
	}
	if len(runner.runCommand.Stdin) != 0 {
		t.Fatalf("command stdin = %q, want empty", string(runner.runCommand.Stdin))
	}
}

func TestSOPS_Encrypt(t *testing.T) {
	// Arrange
	root := t.TempDir()
	targetPath := filepath.Join(root, "env", "api", "dev.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sops.yaml"), []byte("creation_rules: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner := &fakeRunner{
		runResult: &Result{Stdout: []byte("encrypted")},
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	ciphertext, err := adapter.Encrypt(t.Context(), targetPath, []byte("APP_ENV=dev\n"))
	// Assert
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}
	if string(ciphertext) != "encrypted" {
		t.Fatalf("Encrypt() = %q, want encrypted", string(ciphertext))
	}
	if runner.runCommand.Name != "sops" {
		t.Fatalf("command name = %q, want sops", runner.runCommand.Name)
	}
	if len(runner.runCommand.Args) != 5 {
		t.Fatalf("len(command args) = %d, want 5", len(runner.runCommand.Args))
	}
	if runner.runCommand.Args[0] != "encrypt" {
		t.Fatalf("command args[0] = %q, want encrypt", runner.runCommand.Args[0])
	}
	if runner.runCommand.Args[1] != "--config" || runner.runCommand.Args[2] != filepath.Join(root, ".sops.yaml") {
		t.Fatalf("command args[1:3] = %#v, want config flag", runner.runCommand.Args[1:3])
	}
	if runner.runCommand.Args[3] != "--filename-override" || runner.runCommand.Args[4] != "env/api/dev.env" {
		t.Fatalf("command args[3:5] = %#v, want filename override", runner.runCommand.Args[3:5])
	}
	if runner.runCommand.Dir != root {
		t.Fatalf("command dir = %q, want %q", runner.runCommand.Dir, root)
	}
	if string(runner.runCommand.Stdin) != "APP_ENV=dev\n" {
		t.Fatalf("command stdin = %q, want plaintext", string(runner.runCommand.Stdin))
	}
}

func TestSOPS_Rekey(t *testing.T) {
	// Arrange
	root := t.TempDir()
	targetPath := filepath.Join(root, "env", "api", "dev.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sops.yaml"), []byte("creation_rules: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner := &fakeRunner{}
	adapter := NewSOPSWithRunner(runner)

	// Act
	err := adapter.Rekey(t.Context(), targetPath)
	// Assert
	if err != nil {
		t.Fatalf("Rekey() error = %v, want nil", err)
	}
	if runner.runCommand.Name != "sops" {
		t.Fatalf("command name = %q, want sops", runner.runCommand.Name)
	}
	if len(runner.runCommand.Args) != 5 {
		t.Fatalf("len(command args) = %d, want 5", len(runner.runCommand.Args))
	}
	if runner.runCommand.Args[0] != "updatekeys" || runner.runCommand.Args[1] != "--yes" {
		t.Fatalf("command args[:2] = %#v, want updatekeys --yes", runner.runCommand.Args[:2])
	}
	if runner.runCommand.Args[2] != "--config" || runner.runCommand.Args[3] != filepath.Join(root, ".sops.yaml") {
		t.Fatalf("command args[2:4] = %#v, want config flag", runner.runCommand.Args[2:4])
	}
	if runner.runCommand.Args[4] != "env/api/dev.env" {
		t.Fatalf("command args[4] = %q, want env/api/dev.env", runner.runCommand.Args[4])
	}
	if runner.runCommand.Dir != root {
		t.Fatalf("command dir = %q, want %q", runner.runCommand.Dir, root)
	}
}

func TestSOPS_CommandErrors(t *testing.T) {
	exitErr := runExitError(t, 7)

	tests := []struct {
		name    string
		path    string
		result  *Result
		runErr  error
		wantErr string
	}{
		{
			name:    "exit code with stderr",
			path:    "/tmp/dev.env",
			result:  &Result{Stderr: []byte(" failed to decrypt \n\n")},
			runErr:  exitErr,
			wantErr: `run sops decrypt "/tmp/dev.env": exit code 7: failed to decrypt`,
		},
		{
			name:    "stderr without exit error",
			path:    "/tmp/dev.env",
			result:  &Result{Stderr: []byte(" process crashed ")},
			runErr:  errors.New("signal: killed"),
			wantErr: `run sops decrypt "/tmp/dev.env": process crashed`,
		},
		{
			name:    "plain wrapped error",
			path:    "/tmp/dev.env",
			result:  &Result{},
			runErr:  errors.New("timeout"),
			wantErr: `run sops decrypt "/tmp/dev.env": timeout`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			runner := &fakeRunner{
				runResult: tt.result,
				runErr:    tt.runErr,
			}
			adapter := NewSOPSWithRunner(runner)

			// Act
			_, err := adapter.Decrypt(t.Context(), tt.path)

			// Assert
			if err == nil {
				t.Fatal("Decrypt() error = nil, want non-nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Decrypt() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func runExitError(t *testing.T, exitCode int) error {
	t.Helper()

	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd.exe", "/d", "/s", "/c", fmt.Sprintf("exit %d", exitCode))
	} else {
		command = exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", exitCode))
	}

	err := command.Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}

	return fmt.Errorf("run exit helper: %w", err)
}

func TestSOPS_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name string
		run  func(*SOPS) error
		want string
	}{
		{
			name: "decrypt empty path",
			run: func(adapter *SOPS) error {
				_, err := adapter.Decrypt(t.Context(), "")
				return err
			},
			want: "validate target path: empty path",
		},
		{
			name: "encrypt empty path",
			run: func(adapter *SOPS) error {
				_, err := adapter.Encrypt(t.Context(), "", []byte("APP_ENV=dev\n"))
				return err
			},
			want: "validate target path: empty path",
		},
		{
			name: "rekey empty path",
			run: func(adapter *SOPS) error {
				return adapter.Rekey(t.Context(), "")
			},
			want: "validate target path: empty path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			adapter := NewSOPSWithRunner(&fakeRunner{})

			// Act
			err := tt.run(adapter)

			// Assert
			if err == nil {
				t.Fatal("error = nil, want non-nil")
			}
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestSOPS_CheckFailsWhenNotInPath(t *testing.T) {
	// Arrange
	runner := &fakeRunner{
		lookPathErr: errors.New("not found"),
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	err := adapter.Check(t.Context())

	// Assert
	if err == nil {
		t.Fatal("Check() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "locate sops") {
		t.Fatalf("Check() error = %q, want locate sops", err.Error())
	}
}

func TestSOPS_DecryptFailsCheckFirst(t *testing.T) {
	// Arrange
	runner := &fakeRunner{
		lookPathErr: errors.New("not found"),
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	_, err := adapter.Decrypt(t.Context(), "/tmp/dev.env")

	// Assert
	if err == nil {
		t.Fatal("Decrypt() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "locate sops") {
		t.Fatalf("Decrypt() error = %q, want locate sops", err.Error())
	}
}

func TestSOPS_EncryptFailsCheckFirst(t *testing.T) {
	// Arrange
	runner := &fakeRunner{
		lookPathErr: errors.New("not found"),
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	_, err := adapter.Encrypt(t.Context(), "/tmp/dev.env", []byte("data"))

	// Assert
	if err == nil {
		t.Fatal("Encrypt() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "locate sops") {
		t.Fatalf("Encrypt() error = %q, want locate sops", err.Error())
	}
}

func TestSOPS_RekeyFailsCheckFirst(t *testing.T) {
	// Arrange
	runner := &fakeRunner{
		lookPathErr: errors.New("not found"),
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	err := adapter.Rekey(t.Context(), "/tmp/dev.env")

	// Assert
	if err == nil {
		t.Fatal("Rekey() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "locate sops") {
		t.Fatalf("Rekey() error = %q, want locate sops", err.Error())
	}
}

func TestSOPS_EncryptCommandError(t *testing.T) {
	// Arrange
	root := t.TempDir()
	targetPath := filepath.Join(root, "env", "api", "dev.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sops.yaml"), []byte("creation_rules: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exitErr := runExitError(t, 1)
	runner := &fakeRunner{
		runResult: &Result{Stderr: []byte("encrypt failed")},
		runErr:    exitErr,
	}
	adapter := NewSOPSWithRunner(runner)

	//  Act
	_, err := adapter.Encrypt(t.Context(), targetPath, []byte("data"))

	// Assert
	if err == nil {
		t.Fatal("Encrypt() error = nil, want non-nil")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("Encrypt() error type = %T, want *CommandError", err)
	}
	if cmdErr.Action != "encrypt" {
		t.Fatalf("CommandError.Action = %q, want encrypt", cmdErr.Action)
	}
}

func TestSOPS_RekeyCommandError(t *testing.T) {
	// Arrange
	root := t.TempDir()
	targetPath := filepath.Join(root, "env", "api", "dev.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sops.yaml"), []byte("creation_rules: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exitErr := runExitError(t, 1)
	runner := &fakeRunner{
		runResult: &Result{Stderr: []byte("rekey failed")},
		runErr:    exitErr,
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	err := adapter.Rekey(t.Context(), targetPath)

	// Assert
	if err == nil {
		t.Fatal("Rekey() error = nil, want non-nil")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("Rekey() error type = %T, want *CommandError", err)
	}
	if cmdErr.Action != "rekey" {
		t.Fatalf("CommandError.Action = %q, want rekey", cmdErr.Action)
	}
}

func TestNewSOPS_NilRunner(t *testing.T) {
	// Act
	adapter := NewSOPSWithRunner(nil)

	// Assert
	if adapter == nil {
		t.Fatal("NewSOPSWithRunner(nil) = nil, want non-nil")
	}
}

func TestCommandError_ExitCodeOnly(t *testing.T) {
	// Arrange
	exitErr := runExitError(t, 5)
	runner := &fakeRunner{
		runResult: &Result{Stderr: []byte("")},
		runErr:    exitErr,
	}
	adapter := NewSOPSWithRunner(runner)

	// Act
	_, err := adapter.Decrypt(t.Context(), "/tmp/dev.env")

	// Assert
	if err == nil {
		t.Fatal("Decrypt() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "exit code") {
		t.Fatalf("error = %q, want exit code message", err.Error())
	}
}

func TestCommandError_Unwrap(t *testing.T) {
	// Arrange
	inner := errors.New("inner error")

	// Act
	cmdErr := &CommandError{
		Tool:   "sops",
		Action: "decrypt",
		Target: "/tmp/test.env",
		Err:    inner,
	}

	// Assert
	if !errors.Is(cmdErr, inner) {
		t.Fatal("Unwrap() did not return inner error")
	}
}

func TestSanitizeStderr(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"empty", []byte(""), ""},
		{"whitespace only", []byte("  \n\t  "), ""},
		{"trims and collapses", []byte("  hello   world  \n"), "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := sanitizeStderr(tt.input)

			// Assert
			if got != tt.want {
				t.Fatalf("sanitizeStderr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindSOPSConfig_NotFound(t *testing.T) {
	// Arrange
	targetPath := filepath.Join(t.TempDir(), "env", "api", "dev.env")

	// Act
	_, _, err := findSOPSConfig(targetPath)

	// Assert
	if err == nil {
		t.Fatal("findSOPSConfig() error = nil, want non-nil")
	}
	if err.Error() != fmt.Sprintf("lookup sops config for %q: not found", targetPath) {
		t.Fatalf("findSOPSConfig() error = %q, want not found", err.Error())
	}
}
