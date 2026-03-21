package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_NilSchema(t *testing.T) {
	// Arrange
	var s *Schema

	// Act
	err := s.Validate()
	// Assert
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil for nil schema", err)
	}
}

func TestValidate_EmptyKeys(t *testing.T) {
	// Arrange
	s := &Schema{Keys: map[string]Key{}}

	// Act
	err := s.Validate()

	// Assert
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no keys configured") {
		t.Fatalf("Validate() error = %q, want no keys configured", err.Error())
	}
}

func TestValidate_DuplicateEnumValues(t *testing.T) {
	// Arrange
	s := &Schema{
		Keys: map[string]Key{
			"APP_ENV": {Type: "enum", Values: []string{"dev", "dev"}},
		},
	}

	// Act
	err := s.Validate()

	// Assert
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate value") {
		t.Fatalf("Validate() error = %q, want duplicate value failure", err.Error())
	}
}

func TestValidate_EmptyEnumValue(t *testing.T) {
	// Arrange
	s := &Schema{
		Keys: map[string]Key{
			"APP_ENV": {Type: "enum", Values: []string{"dev", ""}},
		},
	}

	// Act
	err := s.Validate()

	// Assert
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "empty value") {
		t.Fatalf("Validate() error = %q, want empty value failure", err.Error())
	}
}

func TestSortedKeys_NilSchema(t *testing.T) {
	// Arrange
	var s *Schema

	// Act
	keys := s.SortedKeys()

	// Assert
	if keys != nil {
		t.Fatalf("SortedKeys() = %v, want nil", keys)
	}
}

func TestSortedKeys_ReturnsSorted(t *testing.T) {
	// Arrange
	s := &Schema{
		Keys: map[string]Key{
			"ZEBRA":  {Type: "string"},
			"ALPHA":  {Type: "string"},
			"MIDDLE": {Type: "string"},
		},
	}

	// Act
	keys := s.SortedKeys()

	// Assert
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	if keys[0] != "ALPHA" || keys[1] != "MIDDLE" || keys[2] != "ZEBRA" {
		t.Fatalf("keys = %v, want [ALPHA MIDDLE ZEBRA]", keys)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	// Act
	_, err := Load("")

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("Load() error = %q, want empty path", err.Error())
	}
}

func TestLoad_MissingFile(t *testing.T) {
	// Act
	_, err := Load("/nonexistent/path/schema.yaml")

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "read schema") {
		t.Fatalf("Load() error = %q, want read schema failure", err.Error())
	}
}

func TestValidate_ValidSchemaTypes(t *testing.T) {
	for _, typ := range []string{"", "string", "bool", "int", "url"} {
		t.Run("type_"+typ, func(t *testing.T) {
			// Arrange
			s := &Schema{
				Keys: map[string]Key{
					"KEY": {Type: typ},
				},
			}

			// Act
			err := s.Validate()
			// Assert
			if err != nil {
				t.Fatalf("Validate() error = %v for type %q", err, typ)
			}
		})
	}
}

func TestKey_ValidateValue_EmptyType(t *testing.T) {
	// Arrange
	k := Key{Type: ""}

	// Act
	err := k.ValidateValue("anything")
	// Assert
	if err != nil {
		t.Fatalf("ValidateValue() error = %v, want nil", err)
	}
}

func TestLoad_ValidSchema(t *testing.T) {
	// Arrange
	root := t.TempDir()
	schemaPath := filepath.Join(root, "api.yaml")
	data := `keys:
  APP_ENV:
    required: true
    type: string
    secret: false
`
	if err := os.WriteFile(schemaPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Act
	s, err := Load(schemaPath)
	// Assert
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if s == nil {
		t.Fatal("Load() = nil, want non-nil")
	}
	if len(s.Keys) != 1 {
		t.Fatalf("len(s.Keys) = %d, want 1", len(s.Keys))
	}
}

func TestLoad_RejectsUnknownFields(t *testing.T) {
	// Arrange
	root := t.TempDir()
	schemaPath := filepath.Join(root, "api.yaml")
	data := `keys:
  APP_ENV:
    required: true
    type: string
    secret: false
    description: unsupported
`
	if err := os.WriteFile(schemaPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Act
	_, err := Load(schemaPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse schema") {
		t.Fatalf("Load() error = %q, want parse schema failure", err.Error())
	}
}

func TestLoad_RejectsMalformedEnum(t *testing.T) {
	// Arrange
	root := t.TempDir()
	schemaPath := filepath.Join(root, "api.yaml")
	data := `keys:
  APP_ENV:
    required: true
    type: enum
    secret: false
`
	if err := os.WriteFile(schemaPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Act
	_, err := Load(schemaPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "enum requires values") {
		t.Fatalf("Load() error = %q, want enum validation failure", err.Error())
	}
}

func TestLoad_RejectsValuesOnNonEnum(t *testing.T) {
	// Arrange
	root := t.TempDir()
	schemaPath := filepath.Join(root, "api.yaml")
	data := `keys:
  APP_ENV:
    required: true
    type: string
    secret: false
    values: [dev]
`
	if err := os.WriteFile(schemaPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Act
	_, err := Load(schemaPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "values require enum type") {
		t.Fatalf("Load() error = %q, want non-enum values failure", err.Error())
	}
}

func TestLoad_RejectsUnsupportedType(t *testing.T) {
	// Arrange
	root := t.TempDir()
	schemaPath := filepath.Join(root, "api.yaml")
	data := `keys:
  APP_ENV:
    required: true
    type: json
    secret: false
`
	if err := os.WriteFile(schemaPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	// Act
	_, err := Load(schemaPath)

	// Assert
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("Load() error = %q, want unsupported type failure", err.Error())
	}
}

func TestKey_ValidateValue(t *testing.T) {
	// Arrange
	tests := []struct {
		name    string
		key     Key
		value   string
		wantErr string
	}{
		{
			name:  "string",
			key:   Key{Type: "string"},
			value: "anything",
		},
		{
			name:  "bool",
			key:   Key{Type: "bool"},
			value: "true",
		},
		{
			name:    "bool invalid",
			key:     Key{Type: "bool"},
			value:   "nope",
			wantErr: "parse bool",
		},
		{
			name:  "int",
			key:   Key{Type: "int"},
			value: "42",
		},
		{
			name:    "int invalid",
			key:     Key{Type: "int"},
			value:   "4.2",
			wantErr: "parse int",
		},
		{
			name:  "url",
			key:   Key{Type: "url"},
			value: "https://example.com/path",
		},
		{
			name:    "url invalid",
			key:     Key{Type: "url"},
			value:   "not-a-url",
			wantErr: "parse url",
		},
		{
			name:  "enum",
			key:   Key{Type: "enum", Values: []string{"dev", "stg"}},
			value: "dev",
		},
		{
			name:    "enum invalid",
			key:     Key{Type: "enum", Values: []string{"dev", "stg"}},
			value:   "prod",
			wantErr: "validate enum",
		},
		{
			name:    "unsupported",
			key:     Key{Type: "json"},
			value:   "value",
			wantErr: "unsupported type",
		},
	}

	// Act & Assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.ValidateValue(tt.value)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateValue() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("ValidateValue() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateValue() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
