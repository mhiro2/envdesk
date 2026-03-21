package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mhiro2/envdesk/internal/atomicwrite"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/envfile"
	"github.com/mhiro2/envdesk/internal/schema"
)

type EditOptions struct {
	Service     string
	Environment string
	Editor      string
	SkipLint    bool
	TempDir     string
}

type EditResult struct {
	Path string `json:"path"`
}

type editRunner interface {
	Run(ctx context.Context, editor, path string) error
}

type shellEditRunner struct{}

func (shellEditRunner) Run(ctx context.Context, editor, path string) error {
	command, err := buildEditorCommand(ctx, editor, path, currentEditorShell())
	if err != nil {
		return err
	}
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("run editor %q: exit code %d: %w", editor, exitErr.ExitCode(), err)
		}

		return fmt.Errorf("run editor %q: %w", editor, err)
	}

	return nil
}

func Edit(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts EditOptions) (*EditResult, error) {
	return editWithRunner(ctx, project, adapter, shellEditRunner{}, opts)
}

func editWithRunner(
	ctx context.Context,
	project *config.Project,
	adapter crypto.Adapter,
	runner editRunner,
	opts EditOptions,
) (*EditResult, error) {
	if project == nil {
		return nil, fmt.Errorf("resolve project: nil project")
	}
	if adapter == nil {
		return nil, fmt.Errorf("resolve crypto adapter: nil adapter")
	}
	if runner == nil {
		return nil, fmt.Errorf("resolve edit runner: nil runner")
	}

	service, err := project.Service(opts.Service)
	if err != nil {
		return nil, fmt.Errorf("lookup service %q: %w", opts.Service, err)
	}

	targetPath, err := service.FilePath(opts.Environment)
	if err != nil {
		return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	editor, err := resolveEditor(opts.Editor)
	if err != nil {
		return nil, err
	}

	plaintext, err := adapter.Decrypt(ctx, targetPath)
	if err != nil {
		return nil, fmt.Errorf("decrypt env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	tempFile, err := os.CreateTemp(opts.TempDir, "envdesk-edit-*.env")
	if err != nil {
		return nil, fmt.Errorf("create temp file for %s/%s: %w", service.Name, opts.Environment, err)
	}
	tempPath := tempFile.Name()

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(plaintext); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("write temp file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	if err := runner.Run(ctx, editor, tempPath); err != nil {
		return nil, fmt.Errorf("edit env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	// #nosec G304 -- the temporary file path is created by this command.
	// #nosec G304 -- the temp file path is created locally in this function.
	edited, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, fmt.Errorf("read temp file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	doc, err := envfile.Parse(edited)
	if err != nil {
		return nil, fmt.Errorf("parse edited env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	if !opts.SkipLint {
		problems, err := lintEditedDocument(service, opts.Environment, doc)
		if err != nil {
			return nil, err
		}
		if len(problems) > 0 {
			return nil, fmt.Errorf("validate edited env file for %s/%s: %s", service.Name, opts.Environment, formatProblems(problems))
		}
	}

	ciphertext, err := adapter.Encrypt(ctx, targetPath, edited)
	if err != nil {
		return nil, fmt.Errorf("encrypt env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	if err := atomicwrite.File(targetPath, ciphertext, 0o600); err != nil {
		return nil, fmt.Errorf("write env file for %s/%s: %w", service.Name, opts.Environment, err)
	}

	return &EditResult{Path: targetPath}, nil
}

func resolveEditor(editor string) (string, error) {
	if trimmed := strings.TrimSpace(editor); trimmed != "" {
		return trimmed, nil
	}

	for _, envVar := range []string{"EDITOR", "VISUAL"} {
		if value := strings.TrimSpace(os.Getenv(envVar)); value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("resolve editor: empty editor")
}

func lintEditedDocument(service config.Service, envName string, doc *envfile.Document) ([]Problem, error) {
	var loadedSchema *schema.Schema
	if service.SchemaPath != "" {
		schemaDoc, err := schema.Load(service.SchemaPath)
		if err != nil {
			return nil, fmt.Errorf("load schema for service %q: %w", service.Name, err)
		}

		loadedSchema = schemaDoc
	}

	return lintDocument(service.Name, envName, doc, loadedSchema), nil
}

func formatProblems(problems []Problem) string {
	parts := make([]string, 0, len(problems))
	for _, problem := range problems {
		part := string(problem.Severity)
		if problem.Key != "" {
			part += " " + problem.Key
		}
		part += ": " + problem.Message
		parts = append(parts, part)
	}

	return strings.Join(parts, "; ")
}
