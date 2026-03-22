package sopsconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocument_UpdateRecipients_NormalizesRecipientWhitespace(t *testing.T) {
	tests := []struct {
		name         string
		recipient    string
		remove       bool
		initialValue string
		wantValues   []string
	}{
		{
			name:         "add trims existing value",
			recipient:    "age1example",
			initialValue: " age1example ",
			wantValues:   []string{"age1example"},
		},
		{
			name:         "remove trims input value",
			recipient:    " age1example ",
			remove:       true,
			initialValue: "age1example",
			wantValues:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := filepath.Join(t.TempDir(), ".sops.yaml")
			data := "creation_rules:\n  - path_regex: ^env/.*\\.env$\n    age:\n      - \"" + tt.initialValue + "\"\n"
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatalf("write sops config: %v", err)
			}

			document, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			// Act
			result, err := document.UpdateRecipients([]string{"env/api/dev.env"}, tt.recipient, tt.remove)
			// Assert
			if err != nil {
				t.Fatalf("UpdateRecipients() error = %v, want nil", err)
			}
			if result.ChangedRules != 1 {
				t.Fatalf("result.ChangedRules = %d, want 1", result.ChangedRules)
			}

			ageNode, ok := mappingValue(document.creationRules.Content[0], "age")
			if !ok {
				t.Fatal("age node missing, want sequence")
			}
			if len(ageNode.Content) != len(tt.wantValues) {
				t.Fatalf("len(ageNode.Content) = %d, want %d", len(ageNode.Content), len(tt.wantValues))
			}
			for idx, value := range tt.wantValues {
				if ageNode.Content[idx].Value != value {
					t.Fatalf("ageNode.Content[%d].Value = %q, want %q", idx, ageNode.Content[idx].Value, value)
				}
			}
		})
	}
}

func TestDocument_UpdateRecipients_RejectsBlankRecipient(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), ".sops.yaml")
	data := "creation_rules:\n  - path_regex: ^env/.*\\.env$\n    age: []\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write sops config: %v", err)
	}

	document, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Act
	_, err = document.UpdateRecipients([]string{"env/api/dev.env"}, "   ", false)

	// Assert
	if err == nil {
		t.Fatal("UpdateRecipients() error = nil, want non-nil")
	}
}
