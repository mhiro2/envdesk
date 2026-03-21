package envfile

import (
	"strings"
	"testing"
)

func TestParse_HandlesCommentsQuotesAndExportPrefix(t *testing.T) {
	// Arrange
	input := "" +
		"# ignored comment\n" +
		"\n" +
		"export   APP_ENV=dev\n" +
		"DATABASE_URL=postgres://user:pass@db.example.com:5432/app?sslmode=disable\n" +
		"ESCAPED=\"line1\\nline2\\t\\\"quote\\\"\\\\tail\"\n" +
		"SINGLE='literal # not comment'\n" +
		"INLINE=value # trailing comment\n"

	// Act
	doc, err := Parse([]byte(input))
	// Assert
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(doc.Entries) != 5 {
		t.Fatalf("len(doc.Entries) = %d, want 5", len(doc.Entries))
	}
	if value, ok := doc.Lookup("APP_ENV"); !ok || value != "dev" {
		t.Fatalf("APP_ENV = %q, %v, want dev", value, ok)
	}
	if value, ok := doc.Lookup("ESCAPED"); !ok || value != "line1\nline2\t\"quote\"\\tail" {
		t.Fatalf("ESCAPED = %q, %v, want decoded escapes", value, ok)
	}
	if value, ok := doc.Lookup("SINGLE"); !ok || value != "literal # not comment" {
		t.Fatalf("SINGLE = %q, %v, want literal hash", value, ok)
	}
	if value, ok := doc.Lookup("INLINE"); !ok || value != "value" {
		t.Fatalf("INLINE = %q, %v, want stripped comment", value, ok)
	}

	want := strings.Join([]string{
		"APP_ENV=dev",
		`DATABASE_URL="postgres://user:pass@db.example.com:5432/app?sslmode=disable"`,
		`ESCAPED="line1\nline2\t\"quote\"\\tail"`,
		`SINGLE="literal # not comment"`,
		"INLINE=value",
		"",
	}, "\n")
	if got := string(doc.Bytes()); got != want {
		t.Fatalf("doc.Bytes() = %q, want %q", got, want)
	}
}

func TestParse_RejectsInvalidSyntax(t *testing.T) {
	// Arrange
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "missing assignment",
			input:   "APP_ENV",
			wantErr: "missing '='",
		},
		{
			name:    "unterminated double quote",
			input:   `APP_ENV="dev`,
			wantErr: "unterminated quoted value",
		},
		{
			name:    "trailing text after quoted value",
			input:   `APP_ENV="dev" extra`,
			wantErr: "unexpected trailing content",
		},
		{
			name:    "invalid key",
			input:   "1APP_ENV=dev",
			wantErr: "invalid key",
		},
		{
			name:    "multiline quoted value",
			input:   "APP_ENV=\"dev\nprod\"",
			wantErr: "unterminated quoted value",
		},
	}

	// Act & Assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatalf("Parse() error = nil, want %q", tt.wantErr)
			}
			if doc != nil {
				t.Fatalf("Parse() document = %#v, want nil", doc)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Parse() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParse_RejectsDuplicateKeys(t *testing.T) {
	// Arrange
	input := "APP_ENV=dev\nAPP_ENV=stg\n"

	// Act
	doc, err := Parse([]byte(input))

	// Assert
	if err == nil {
		t.Fatal("Parse() error = nil, want non-nil")
	}
	if doc != nil {
		t.Fatalf("Parse() document = %#v, want nil", doc)
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("Parse() error = %q, want duplicate key failure", err.Error())
	}
}
