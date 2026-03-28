package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const sopsConfigName = ".sops.yaml"

type Command struct {
	Name  string
	Args  []string
	Dir   string
	Stdin []byte
}

type Result struct {
	Stdout []byte
	Stderr []byte
}

type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, cmd Command) (*Result, error)
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return "", fmt.Errorf("look up executable %q: %w", file, err)
	}

	return path, nil
}

func (execRunner) Run(ctx context.Context, cmd Command) (*Result, error) {
	// #nosec G204 -- the adapter constructs a fixed command shape for the target tool.
	command := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	command.Dir = cmd.Dir
	command.Stdin = bytes.NewReader(cmd.Stdin)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		err = fmt.Errorf("run command %q: %w", cmd.Name, err)
	}

	return &Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}, err
}

type SOPS struct {
	runner   Runner
	repoRoot string
}

func NewSOPS() *SOPS {
	return NewSOPSWithRunner(execRunner{})
}

func NewSOPSForRepo(repoRoot string) *SOPS {
	return &SOPS{runner: execRunner{}, repoRoot: repoRoot}
}

func NewSOPSWithRunner(runner Runner) *SOPS {
	if runner == nil {
		runner = execRunner{}
	}

	return &SOPS{runner: runner}
}

func (s *SOPS) Check(_ context.Context) error {
	if _, err := s.runner.LookPath("sops"); err != nil {
		return fmt.Errorf("locate sops: %w", err)
	}

	return nil
}

func (s *SOPS) Decrypt(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	targetPath, err := validatePath(path)
	if err != nil {
		return nil, err
	}
	if err := s.Check(ctx); err != nil {
		return nil, err
	}

	result, runErr := s.runner.Run(ctx, Command{
		Name: "sops",
		Args: []string{"decrypt", targetPath},
		Dir:  filepath.Dir(targetPath),
	})
	if runErr != nil {
		return nil, wrapCommandError("decrypt", targetPath, result, runErr)
	}

	return result.Stdout, nil
}

func (s *SOPS) Encrypt(ctx context.Context, path string, plaintext []byte) ([]byte, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	targetPath, err := validatePath(path)
	if err != nil {
		return nil, err
	}
	if err := s.Check(ctx); err != nil {
		return nil, err
	}

	configPath, relativeTarget, err := findSOPSConfig(targetPath, s.repoRoot)
	if err != nil {
		return nil, err
	}

	result, runErr := s.runner.Run(ctx, Command{
		Name: "sops",
		Args: []string{
			// --config is a global flag and must appear before the subcommand (sops 3.9+).
			"--config", configPath,
			"encrypt",
			"--filename-override", filepath.ToSlash(relativeTarget),
			// Required for stdin encrypt: filename extension alone is not always enough (see sops README).
			"--input-type", "dotenv",
			"--output-type", "dotenv",
		},
		Dir:   filepath.Dir(configPath),
		Stdin: plaintext,
	})
	if runErr != nil {
		return nil, wrapCommandError("encrypt", targetPath, result, runErr)
	}

	return result.Stdout, nil
}

func (s *SOPS) Rekey(ctx context.Context, path string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	targetPath, err := validatePath(path)
	if err != nil {
		return err
	}
	if err := s.Check(ctx); err != nil {
		return err
	}

	configPath, relativeTarget, err := findSOPSConfig(targetPath, s.repoRoot)
	if err != nil {
		return err
	}

	result, runErr := s.runner.Run(ctx, Command{
		Name: "sops",
		Args: []string{
			"--config", configPath,
			"updatekeys",
			"--yes",
			filepath.ToSlash(relativeTarget),
		},
		Dir: filepath.Dir(configPath),
	})
	if runErr != nil {
		return wrapCommandError("rekey", targetPath, result, runErr)
	}

	return nil
}

func wrapCommandError(action, target string, result *Result, err error) error {
	if result == nil {
		result = &Result{}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &CommandError{
			Tool:     "sops",
			Action:   action,
			Target:   target,
			ExitCode: exitErr.ExitCode(),
			Stderr:   sanitizeStderr(result.Stderr),
			Err:      err,
		}
	}

	stderr := sanitizeStderr(result.Stderr)
	if stderr != "" {
		return &CommandError{
			Tool:   "sops",
			Action: action,
			Target: target,
			Stderr: stderr,
			Err:    err,
		}
	}

	return fmt.Errorf("run sops %s %q: %w", action, target, err)
}

func validatePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("validate target path: empty path")
	}

	return cleaned, nil
}

func findSOPSConfig(targetPath, repoRoot string) (string, string, error) {
	searchDir := filepath.Dir(targetPath)
	boundary := filepath.Clean(repoRoot)

	for {
		configPath := filepath.Join(searchDir, sopsConfigName)
		info, err := os.Stat(configPath)
		if err == nil {
			if info.IsDir() {
				return "", "", fmt.Errorf("lookup sops config for %q: %s is a directory", targetPath, sopsConfigName)
			}

			relativeTarget, relErr := filepath.Rel(searchDir, targetPath)
			if relErr != nil {
				return "", "", fmt.Errorf("resolve sops target %q: %w", targetPath, relErr)
			}

			return configPath, relativeTarget, nil
		}
		if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("lookup sops config for %q: %w", targetPath, err)
		}

		// Stop at the repository root to avoid inheriting a parent workspace's .sops.yaml.
		if boundary != "" && boundary != "." && searchDir == boundary {
			break
		}

		parentDir := filepath.Dir(searchDir)
		if parentDir == searchDir {
			break
		}

		searchDir = parentDir
	}

	return "", "", fmt.Errorf("lookup sops config for %q: not found", targetPath)
}
