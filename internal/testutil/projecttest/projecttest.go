package projecttest

import (
	"os"
	"path/filepath"
	"testing"
)

func WriteProject(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for path, content := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", fullPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}

	return root
}
