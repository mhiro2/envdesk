package schema

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Schema struct {
	Keys map[string]Key `yaml:"keys"`
}

type Key struct {
	Required bool     `yaml:"required"`
	Type     string   `yaml:"type"`
	Secret   bool     `yaml:"secret"`
	Values   []string `yaml:"values"`
}

func Load(path string) (*Schema, error) {
	if path == "" {
		return nil, fmt.Errorf("load schema path: empty path")
	}

	// #nosec G304 -- schema path is resolved from explicit project configuration.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %q: %w", path, err)
	}

	var loaded Schema
	if err := decodeYAML(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse schema %q: %w", path, err)
	}

	if err := loaded.Validate(); err != nil {
		return nil, fmt.Errorf("validate schema %q: %w", path, err)
	}

	return &loaded, nil
}

func (s *Schema) Validate() error {
	if s == nil {
		return nil
	}

	if len(s.Keys) == 0 {
		return fmt.Errorf("validate keys: no keys configured")
	}

	for name, key := range s.Keys {
		if name == "" {
			return fmt.Errorf("validate key name: empty name")
		}

		switch key.Type {
		case "", "string", "bool", "int", "url":
			if len(key.Values) > 0 {
				return fmt.Errorf("validate key %q: values require enum type", name)
			}
		case "enum":
			if len(key.Values) == 0 {
				return fmt.Errorf("validate key %q: enum requires values", name)
			}

			seen := make(map[string]struct{}, len(key.Values))
			for _, value := range key.Values {
				if value == "" {
					return fmt.Errorf("validate key %q: enum contains empty value", name)
				}

				if _, exists := seen[value]; exists {
					return fmt.Errorf("validate key %q: enum contains duplicate value %q", name, value)
				}

				seen[value] = struct{}{}
			}
		default:
			return fmt.Errorf("validate key %q type %q: unsupported type", name, key.Type)
		}
	}

	return nil
}

func (s *Schema) SortedKeys() []string {
	if s == nil {
		return nil
	}

	keys := make([]string, 0, len(s.Keys))
	for name := range s.Keys {
		keys = append(keys, name)
	}

	slices.Sort(keys)

	return keys
}

func (k Key) ValidateValue(value string) error {
	switch k.Type {
	case "", "string":
		return nil
	case "bool":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("parse bool: %w", err)
		}
	case "int":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("parse int: %w", err)
		}
	case "url":
		parsed, err := url.ParseRequestURI(value)
		if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("parse url: invalid url")
		}
	case "enum":
		if !slices.Contains(k.Values, value) {
			return fmt.Errorf("validate enum: expected one of %v", k.Values)
		}
	default:
		return fmt.Errorf("validate type %q: unsupported type", k.Type)
	}

	return nil
}

func decodeYAML(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode yaml: %w", err)
	}

	return nil
}
