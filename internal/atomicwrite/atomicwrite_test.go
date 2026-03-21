package atomicwrite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhiro2/envdesk/internal/testutil/platformtest"
)

func TestFile_OverwritesTargetAtomically(t *testing.T) {
	// Arrange
	root := t.TempDir()
	targetPath := filepath.Join(root, "env", "api.env")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("before\n"), 0o600); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	// Act
	err := File(targetPath, []byte("after\n"), 0o640)
	// Assert
	if err != nil {
		t.Fatalf("File() error = %v, want nil", err)
	}

	data, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(data) != "after\n" {
		t.Fatalf("target file = %q, want overwritten content", string(data))
	}

	info, statErr := os.Stat(targetPath)
	if statErr != nil {
		t.Fatalf("stat target file: %v", statErr)
	}
	if !platformtest.SupportsExactFileModes() {
		return
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("target mode = %o, want 640", info.Mode().Perm())
	}
}
