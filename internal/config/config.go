package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const currentVersion = 1

var errPathOutsideRepository = errors.New("outside repository")

type rawConfig struct {
	Version  int          `yaml:"version"`
	Services []rawService `yaml:"services"`
}

type rawService struct {
	Name   string            `yaml:"name"`
	Schema string            `yaml:"schema"`
	Files  map[string]string `yaml:"files"`
}

type Project struct {
	ConfigPath string
	BaseDir    string
	Services   map[string]Service
}

type Service struct {
	Name       string
	SchemaPath string
	Files      map[string]string
}

func Load(path string) (*Project, error) {
	cleanedPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("resolve config %q: %w", path, err)
	}

	// #nosec G304 -- config path is selected explicitly by the caller.
	data, err := os.ReadFile(cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", cleanedPath, err)
	}

	var raw rawConfig

	if err := decodeYAML(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", cleanedPath, err)
	}

	if raw.Version != currentVersion {
		return nil, fmt.Errorf("validate config version %d: unsupported version", raw.Version)
	}

	if len(raw.Services) == 0 {
		return nil, fmt.Errorf("validate config services: no services configured")
	}

	baseDir := filepath.Dir(cleanedPath)
	project := &Project{
		ConfigPath: cleanedPath,
		BaseDir:    baseDir,
		Services:   make(map[string]Service, len(raw.Services)),
	}

	for _, service := range raw.Services {
		if service.Name == "" {
			return nil, fmt.Errorf("validate service name: empty name")
		}

		if _, exists := project.Services[service.Name]; exists {
			return nil, fmt.Errorf("validate service %q: duplicate service", service.Name)
		}

		if len(service.Files) == 0 {
			return nil, fmt.Errorf("validate service %q files: no environments configured", service.Name)
		}

		files := make(map[string]string, len(service.Files))
		resolvedFiles := make(map[string]string, len(service.Files))
		for envName, filePath := range service.Files {
			if envName == "" {
				return nil, fmt.Errorf("validate service %q files: empty environment name", service.Name)
			}

			if filePath == "" {
				return nil, fmt.Errorf("validate service %q file %q: empty path", service.Name, envName)
			}

			resolvedPath, err := resolveRepoPath(baseDir, filePath)
			if err != nil {
				return nil, fmt.Errorf("validate service %q file %q: %w", service.Name, filePath, err)
			}
			if previousEnv, exists := resolvedFiles[resolvedPath]; exists {
				return nil, fmt.Errorf(
					"validate service %q files: duplicate env file %q for %q and %q",
					service.Name,
					resolvedPath,
					previousEnv,
					envName,
				)
			}

			resolvedFiles[resolvedPath] = envName
			files[envName] = resolvedPath
		}

		schemaPath, err := resolveRepoPath(baseDir, service.Schema)
		if err != nil {
			return nil, fmt.Errorf("validate service %q schema %q: %w", service.Name, service.Schema, err)
		}
		if service.Schema != "" {
			if err := validateSchemaReference(schemaPath, service.Name, service.Schema); err != nil {
				return nil, err
			}
		}

		project.Services[service.Name] = Service{
			Name:       service.Name,
			SchemaPath: schemaPath,
			Files:      files,
		}
	}

	return project, nil
}

func decodeYAML(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode yaml: %w", err)
	}

	return nil
}

func (p *Project) Service(name string) (Service, error) {
	service, ok := p.Services[name]
	if !ok {
		return Service{}, fmt.Errorf("lookup service %q: not found", name)
	}

	return service, nil
}

func (s Service) Environments() []string {
	envs := make([]string, 0, len(s.Files))
	for envName := range s.Files {
		envs = append(envs, envName)
	}

	slices.Sort(envs)

	return envs
}

func (s Service) FilePath(envName string) (string, error) {
	filePath, ok := s.Files[envName]
	if !ok {
		return "", fmt.Errorf("lookup environment %q for service %q: not found", envName, s.Name)
	}

	return filePath, nil
}

func resolvePath(baseDir, path string) string {
	if path == "" {
		return ""
	}

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Clean(filepath.Join(baseDir, path))
}

func resolveRepoPath(baseDir, path string) (string, error) {
	resolvedPath := resolvePath(baseDir, path)
	if resolvedPath == "" {
		return "", nil
	}

	if err := validateRepoPath(baseDir, resolvedPath); err != nil {
		return "", err
	}

	return resolvedPath, nil
}

func validateRepoPath(baseDir, path string) error {
	relative, err := filepath.Rel(baseDir, path)
	if err != nil {
		return fmt.Errorf("check repository boundary: %w", err)
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errPathOutsideRepository
	}

	return nil
}

func validateSchemaReference(path, serviceName, ref string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("validate service %q schema %q: not found", serviceName, ref)
		}

		return fmt.Errorf("validate service %q schema %q: %w", serviceName, ref, err)
	}

	if info.IsDir() {
		return fmt.Errorf("validate service %q schema %q: is a directory", serviceName, ref)
	}

	return nil
}
