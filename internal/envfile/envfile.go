package envfile

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Document struct {
	Entries []Entry
}

func Load(path string) (*Document, error) {
	// #nosec G304 -- env file path is resolved from explicit project configuration.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read env file %q: %w", path, err)
	}

	doc, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse env file %q: %w", path, err)
	}

	return doc, nil
}

func Parse(data []byte) (*Document, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	doc := &Document{
		Entries: make([]Entry, 0),
	}
	seen := make(map[string]int)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		entry, ok, err := parseLine(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}

		if !ok {
			continue
		}

		if prevLine, exists := seen[entry.Key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key %q (first defined on line %d)", lineNo, entry.Key, prevLine)
		}

		seen[entry.Key] = lineNo
		doc.Entries = append(doc.Entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env file: %w", err)
	}

	return doc, nil
}

func (d *Document) Keys() []string {
	keys := make([]string, 0, len(d.Entries))
	for _, entry := range d.Entries {
		keys = append(keys, entry.Key)
	}

	return keys
}

func (d *Document) Values() map[string]string {
	values := make(map[string]string, len(d.Entries))
	for _, entry := range d.Entries {
		values[entry.Key] = entry.Value
	}

	return values
}

func (d *Document) Lookup(key string) (string, bool) {
	for _, entry := range d.Entries {
		if entry.Key == key {
			return entry.Value, true
		}
	}

	return "", false
}

func (d *Document) Write(path string) error {
	if err := os.WriteFile(path, d.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write env file %q: %w", path, err)
	}

	return nil
}

func (d *Document) Bytes() []byte {
	var builder strings.Builder

	for _, entry := range d.Entries {
		builder.WriteString(entry.Key)
		builder.WriteByte('=')
		builder.WriteString(formatValue(entry.Value))
		builder.WriteByte('\n')
	}

	return []byte(builder.String())
}

func parseLine(line string) (Entry, bool, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return Entry{}, false, nil
	}

	if withoutExport, ok := stripExportPrefix(trimmed); ok {
		trimmed = withoutExport
	}

	key, value, found := strings.Cut(trimmed, "=")
	if !found {
		return Entry{}, false, fmt.Errorf("parse assignment: missing '='")
	}

	key = strings.TrimSpace(key)
	if !isValidKey(key) {
		return Entry{}, false, fmt.Errorf("parse key %q: invalid key", key)
	}

	parsedValue, err := parseValue(strings.TrimSpace(value))
	if err != nil {
		return Entry{}, false, fmt.Errorf("parse value for %q: %w", key, err)
	}

	return Entry{Key: key, Value: parsedValue}, true, nil
}

func parseValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	if raw[0] == '"' {
		value, rest, err := consumeQuoted(raw, '"')
		if err != nil {
			return "", err
		}

		if !isOnlyComment(rest) {
			return "", fmt.Errorf("unexpected trailing content")
		}

		unquoted, err := strconv.Unquote(`"` + value + `"`)
		if err != nil {
			return "", fmt.Errorf("decode quoted value: %w", err)
		}

		return unquoted, nil
	}

	if raw[0] == '\'' {
		value, rest, err := consumeQuoted(raw, '\'')
		if err != nil {
			return "", err
		}

		if !isOnlyComment(rest) {
			return "", fmt.Errorf("unexpected trailing content")
		}

		return value, nil
	}

	if value, ok := splitInlineComment(raw); ok {
		return value, nil
	}

	return strings.TrimSpace(raw), nil
}

func stripExportPrefix(line string) (string, bool) {
	if !strings.HasPrefix(line, "export") {
		return line, false
	}

	rest := line[len("export"):]
	if rest == "" {
		return line, false
	}

	if rest[0] != ' ' && rest[0] != '\t' {
		return line, false
	}

	return strings.TrimSpace(rest), true
}

func splitInlineComment(raw string) (string, bool) {
	for idx := 0; idx < len(raw); idx++ {
		if raw[idx] != '#' {
			continue
		}
		if idx == 0 {
			return "", false
		}

		if isWhitespaceByte(raw[idx-1]) {
			return strings.TrimSpace(raw[:idx]), true
		}
	}

	return "", false
}

func isWhitespaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func consumeQuoted(raw string, quote byte) (string, string, error) {
	var builder strings.Builder
	escaped := false

	for idx := 1; idx < len(raw); idx++ {
		ch := raw[idx]
		if quote == '"' && ch == '\\' && !escaped {
			escaped = true
			builder.WriteByte(ch)
			continue
		}

		if ch == quote && !escaped {
			return builder.String(), raw[idx+1:], nil
		}

		escaped = false
		builder.WriteByte(ch)
	}

	return "", "", fmt.Errorf("unterminated quoted value")
}

func isOnlyComment(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

func isValidKey(key string) bool {
	if key == "" {
		return false
	}

	for idx, r := range key {
		if idx == 0 {
			if !isLetter(r) && r != '_' {
				return false
			}

			continue
		}

		if !isLetter(r) && !isDigit(r) && r != '_' {
			return false
		}
	}

	return true
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func formatValue(value string) string {
	if value == "" {
		return ""
	}

	if isBareValue(value) {
		return value
	}

	return strconv.Quote(value)
}

func isBareValue(value string) bool {
	if strings.TrimSpace(value) != value {
		return false
	}

	for _, r := range value {
		switch {
		case isLetter(r), isDigit(r):
		case strings.ContainsRune("_-./:@%+,", r):
		default:
			return false
		}
	}

	return true
}
