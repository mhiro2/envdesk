package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhiro2/envdesk/internal/crypto"
)

const (
	defaultServiceName = "api"
	configVersion      = 1
)

var defaultEnvironmentNames = []string{"dev", "stg", "prod"}

type InitOptions struct {
	ConfigPath    string
	Services      []string
	Environments  []string
	Force         bool
	ScaffoldSOPS  bool
	Encrypt       bool
	AgeRecipients []string
}

type InitResult struct {
	Files []InitFile `json:"files"`
}

type InitFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func Init(ctx context.Context, adapter crypto.Adapter, opts InitOptions) (*InitResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	configPath := filepath.Clean(opts.ConfigPath)
	if configPath == "" {
		return nil, fmt.Errorf("resolve config path: empty path")
	}
	if opts.Encrypt {
		if adapter == nil {
			return nil, fmt.Errorf("initialize project: missing crypto adapter")
		}
		if len(opts.AgeRecipients) == 0 {
			return nil, fmt.Errorf("initialize project: encrypt mode requires at least one age recipient")
		}
		opts.ScaffoldSOPS = true
	}

	services, err := normalizeScaffoldNames(opts.Services, []string{defaultServiceName}, "services")
	if err != nil {
		return nil, err
	}

	environments, err := normalizeScaffoldNames(opts.Environments, defaultEnvironmentNames, "environments")
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Dir(configPath)
	plans := buildInitPlans(baseDir, configPath, services, environments, opts.ScaffoldSOPS, opts.AgeRecipients)
	if err := ensureInitTargets(plans, opts.Force); err != nil {
		return nil, err
	}

	files := make([]InitFile, 0, len(plans))
	for _, plan := range plans {
		action, err := writeInitPlan(ctx, adapter, plan, opts.Encrypt)
		if err != nil {
			return nil, err
		}

		files = append(files, InitFile{
			Path:   plan.RelPath,
			Action: action,
		})
	}

	return &InitResult{Files: files}, nil
}

type initPlan struct {
	RelPath string
	AbsPath string
	Data    []byte
	Perm    os.FileMode
	EnvFile bool
}

func buildInitPlans(baseDir, configPath string, services, environments []string, scaffoldSOPS bool, ageRecipients []string) []initPlan {
	plans := []initPlan{
		{
			RelPath: relPath(baseDir, configPath),
			AbsPath: configPath,
			Data:    buildConfigFile(services, environments),
			Perm:    0o644,
		},
	}

	for _, service := range services {
		schemaRelPath := filepath.ToSlash(filepath.Join("env.schema", service+".yaml"))
		plans = append(plans, initPlan{
			RelPath: schemaRelPath,
			AbsPath: filepath.Join(baseDir, filepath.FromSlash(schemaRelPath)),
			Data:    buildSchemaFile(environments),
			Perm:    0o644,
		})

		for _, environment := range environments {
			envRelPath := filepath.ToSlash(filepath.Join("env", service, environment+".env"))
			plans = append(plans, initPlan{
				RelPath: envRelPath,
				AbsPath: filepath.Join(baseDir, filepath.FromSlash(envRelPath)),
				Data:    buildEnvFile(environment),
				Perm:    0o600,
				EnvFile: true,
			})
		}
	}

	if scaffoldSOPS {
		sopsRelPath := ".sops.yaml"
		plans = append(plans, initPlan{
			RelPath: sopsRelPath,
			AbsPath: filepath.Join(baseDir, sopsRelPath),
			Data:    buildSOPSFile(ageRecipients),
			Perm:    0o644,
		})
	}

	return plans
}

func ensureInitTargets(plans []initPlan, force bool) error {
	if force {
		return nil
	}

	for _, plan := range plans {
		info, err := os.Stat(plan.AbsPath)
		if err == nil {
			targetType := "file"
			if info.IsDir() {
				targetType = "directory"
			}

			return fmt.Errorf("check scaffold target %q: %s already exists", plan.RelPath, targetType)
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("check scaffold target %q: %w", plan.RelPath, err)
		}
	}

	return nil
}

func writeInitPlan(ctx context.Context, adapter crypto.Adapter, plan initPlan, encrypt bool) (string, error) {
	action := "created"
	if _, err := os.Stat(plan.AbsPath); err == nil {
		action = "overwrote"
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check scaffold target %q: %w", plan.RelPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(plan.AbsPath), 0o750); err != nil {
		return "", fmt.Errorf("create parent directory for %q: %w", plan.RelPath, err)
	}

	data := plan.Data
	if encrypt && plan.EnvFile {
		ciphertext, err := adapter.Encrypt(ctx, plan.AbsPath, plan.Data)
		if err != nil {
			return "", fmt.Errorf("encrypt scaffold file %q: %w", plan.RelPath, err)
		}
		data = ciphertext
	}

	if err := os.WriteFile(plan.AbsPath, data, plan.Perm); err != nil {
		return "", fmt.Errorf("write scaffold file %q: %w", plan.RelPath, err)
	}

	return action, nil
}

func buildConfigFile(services, environments []string) []byte {
	var builder strings.Builder

	builder.WriteString("version: ")
	builder.WriteString(strconv.Itoa(configVersion))
	builder.WriteString("\n\nservices:\n")
	for idx, service := range services {
		builder.WriteString("  - name: ")
		builder.WriteString(service)
		builder.WriteByte('\n')
		builder.WriteString("    schema: env.schema/")
		builder.WriteString(service)
		builder.WriteString(".yaml\n")
		builder.WriteString("    files:\n")
		for _, environment := range environments {
			builder.WriteString("      ")
			builder.WriteString(environment)
			builder.WriteString(": env/")
			builder.WriteString(service)
			builder.WriteByte('/')
			builder.WriteString(environment)
			builder.WriteString(".env\n")
		}

		if idx < len(services)-1 {
			builder.WriteByte('\n')
		}
	}

	return []byte(builder.String())
}

func buildSchemaFile(environments []string) []byte {
	var builder strings.Builder

	builder.WriteString("keys:\n")
	builder.WriteString("  APP_ENV:\n")
	builder.WriteString("    required: true\n")
	builder.WriteString("    type: enum\n")
	builder.WriteString("    values: [")
	builder.WriteString(strings.Join(environments, ", "))
	builder.WriteString("]\n")
	builder.WriteString("    secret: false\n")

	return []byte(builder.String())
}

func buildEnvFile(environment string) []byte {
	return []byte("APP_ENV=" + environment + "\n")
}

func buildSOPSFile(ageRecipients []string) []byte {
	var builder strings.Builder
	builder.WriteString("creation_rules:\n")
	builder.WriteString("  - path_regex: ^env/.*\\.env$\n")
	if len(ageRecipients) == 0 {
		builder.WriteString("    age: []\n")
		return []byte(builder.String())
	}

	builder.WriteString("    age:\n")
	for _, recipient := range ageRecipients {
		builder.WriteString("      - ")
		builder.WriteString(recipient)
		builder.WriteByte('\n')
	}

	return []byte(builder.String())
}

func normalizeScaffoldNames(values, defaults []string, target string) ([]string, error) {
	if len(values) == 0 {
		values = defaults
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("normalize %s: empty value", target)
		}
		if !isScaffoldName(name) {
			return nil, fmt.Errorf("normalize %s: invalid name %q", target, name)
		}
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}

	return normalized, nil
}

func isScaffoldName(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}

		return false
	}

	return true
}

func relPath(baseDir, path string) string {
	relative, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(relative)
}
