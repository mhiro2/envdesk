package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/schema"
)

func loadProject(cmd *cobra.Command) (*config.Project, error) {
	configPath, err := readConfigPath(cmd)
	if err != nil {
		return nil, err
	}

	project, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	return project, nil
}

func readConfigPath(cmd *cobra.Command) (string, error) {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", fmt.Errorf("read config flag: %w", err)
	}

	return configPath, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}

	return nil
}

func formatMetadata(key *schema.Key) string {
	if key == nil {
		return ""
	}

	parts := make([]string, 0, 3)
	if key.Required {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}

	if key.Secret {
		parts = append(parts, "secret")
	} else {
		parts = append(parts, "public")
	}

	if key.Type != "" {
		parts = append(parts, key.Type)
	}

	return " [" + strings.Join(parts, ", ") + "]"
}

func isQuiet(cmd *cobra.Command) bool {
	q, _ := cmd.Flags().GetBool("quiet")
	return q
}

// PrintErrorHint prints a contextual hint for common errors to help users fix issues.
func PrintErrorHint(w io.Writer, err error) {
	if err == nil {
		return
	}
	hint := errorHint(err)
	if hint != "" {
		_, _ = fmt.Fprintf(w, "hint: %s\n", hint)
	}
}

func errorHint(err error) string {
	msg := err.Error()
	hints := []struct {
		pattern string
		hint    string
	}{
		{"locate sops", "install sops: brew install sops (macOS) or see https://github.com/getsops/sops"},
		{`look up executable "sops"`, "install sops: brew install sops (macOS) or see https://github.com/getsops/sops"},
		{`look up executable "age"`, "install age: brew install age (macOS) or see https://github.com/FiloSottile/age"},
		{"lookup sops config", "create a .sops.yaml file or run: envdesk init --sops"},
		{"read config", "create config with: envdesk init"},
		{"no services configured", "add services to envdesk.yaml"},
	}
	for _, h := range hints {
		if strings.Contains(msg, h.pattern) {
			return h.hint
		}
	}
	return ""
}

func formatProjectPath(baseDir, path string) string {
	relative, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(relative)
}
