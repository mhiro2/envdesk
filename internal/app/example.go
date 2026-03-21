package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/envfile"
	"github.com/mhiro2/envdesk/internal/schema"
)

type ExampleGenerateOptions struct {
	Service string
	Out     string
	Force   bool
}

type ExampleGenerateResult struct {
	Files []ExampleGenerateFile `json:"files"`
}

type ExampleGenerateFile struct {
	Service string `json:"service"`
	Path    string `json:"path"`
	Action  string `json:"action"`
}

func ExampleGenerate(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts ExampleGenerateOptions) (*ExampleGenerateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if project == nil {
		return nil, fmt.Errorf("resolve project: nil project")
	}
	if adapter == nil {
		return nil, fmt.Errorf("resolve crypto adapter: nil adapter")
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}
	if opts.Out != "" && len(services) > 1 {
		return nil, fmt.Errorf("select example output: use --service with --out")
	}

	result := &ExampleGenerateResult{
		Files: make([]ExampleGenerateFile, 0, len(services)),
	}

	for _, service := range services {
		targetPath := opts.Out
		if targetPath == "" {
			targetPath, err = defaultExamplePath(service)
			if err != nil {
				return nil, err
			}
		}

		content, err := buildExampleContent(ctx, adapter, service)
		if err != nil {
			return nil, err
		}

		action, err := writeExampleFile(targetPath, content, opts.Force)
		if err != nil {
			return nil, fmt.Errorf("write example file for %s: %w", service.Name, err)
		}

		result.Files = append(result.Files, ExampleGenerateFile{
			Service: service.Name,
			Path:    targetPath,
			Action:  action,
		})
	}

	return result, nil
}

func buildExampleContent(ctx context.Context, adapter crypto.Adapter, service config.Service) ([]byte, error) {
	schemaKeys, schemaMeta, err := loadExampleSchemaKeys(service)
	if err != nil {
		return nil, err
	}

	envKeys, err := loadExampleEnvironmentKeys(ctx, adapter, service)
	if err != nil && len(schemaKeys) == 0 {
		return nil, err
	}

	keys := mergeExampleKeys(schemaKeys, envKeys)
	var builder strings.Builder
	for _, key := range keys {
		if meta, ok := schemaMeta[key]; ok {
			builder.WriteString(exampleMetadataComment(meta))
			if len(meta.Values) > 0 {
				builder.WriteString("# allowed: ")
				builder.WriteString(strings.Join(meta.Values, ", "))
				builder.WriteByte('\n')
			}
		}
		builder.WriteString(key)
		builder.WriteString("=\n")
	}

	return []byte(builder.String()), nil
}

func loadExampleSchemaKeys(service config.Service) ([]string, map[string]schema.Key, error) {
	if service.SchemaPath == "" {
		return nil, nil, nil
	}

	loadedSchema, err := schema.Load(service.SchemaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load schema for service %q: %w", service.Name, err)
	}

	return loadedSchema.SortedKeys(), loadedSchema.Keys, nil
}

func loadExampleEnvironmentKeys(ctx context.Context, adapter crypto.Adapter, service config.Service) ([]string, error) {
	keys := make([]string, 0)
	seen := make(map[string]struct{}, len(service.Files))

	for _, envName := range service.Environments() {
		filePath, err := service.FilePath(envName)
		if err != nil {
			return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
		}

		plaintext, err := adapter.Decrypt(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("decrypt env file for %s/%s: %w", service.Name, envName, err)
		}

		doc, err := envfile.Parse(plaintext)
		if err != nil {
			return nil, fmt.Errorf("parse env file for %s/%s: %w", service.Name, envName, err)
		}

		for _, key := range doc.Keys() {
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func mergeExampleKeys(schemaKeys, envKeys []string) []string {
	if len(schemaKeys) == 0 {
		return envKeys
	}

	keys := make([]string, 0, len(schemaKeys)+len(envKeys))
	seen := make(map[string]struct{}, len(schemaKeys)+len(envKeys))

	for _, key := range schemaKeys {
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	for _, key := range envKeys {
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

func defaultExamplePath(service config.Service) (string, error) {
	envs := service.Environments()
	if len(envs) == 0 {
		return "", fmt.Errorf("select example output for %s: no env files configured", service.Name)
	}

	firstPath, err := service.FilePath(envs[0])
	if err != nil {
		return "", fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envs[0], err)
	}

	return filepath.Join(filepath.Dir(firstPath), ".env.example"), nil
}

func writeExampleFile(path string, data []byte, force bool) (string, error) {
	existed := false
	if _, err := os.Stat(path); err == nil {
		existed = true
	}

	if err := ensureExampleTarget(path, force); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create parent directory for %q: %w", path, err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write example file %q: %w", path, err)
	}

	if existed {
		return "overwrote", nil
	}

	return "created", nil
}

func ensureExampleTarget(path string, force bool) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("check example target %q: directory already exists", path)
		}
		if !force {
			return fmt.Errorf("check example target %q: file already exists", path)
		}

		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("check example target %q: %w", path, err)
	}

	return nil
}

func exampleMetadataComment(meta schema.Key) string {
	parts := make([]string, 0, 3)
	if meta.Required {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}
	if meta.Type != "" {
		parts = append(parts, "type="+meta.Type)
	}
	if meta.Secret {
		parts = append(parts, "secret=true")
	} else {
		parts = append(parts, "secret=false")
	}

	return "# " + strings.Join(parts, ", ") + "\n"
}
