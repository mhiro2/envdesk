package atomicwrite

import (
	"fmt"
	"os"
	"path/filepath"
)

func File(path string, data []byte, perm os.FileMode) error {
	cleanedPath := filepath.Clean(path)
	dir := filepath.Dir(cleanedPath)

	tempFile, err := os.CreateTemp(dir, ".envdesk-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file %q: %w", cleanedPath, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := setTempFileMode(tempFile, perm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("set temp file mode %q: %w", cleanedPath, err)
	}

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp file %q: %w", cleanedPath, err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temp file %q: %w", cleanedPath, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file %q: %w", cleanedPath, err)
	}

	if err := replaceFile(tempPath, cleanedPath); err != nil {
		return fmt.Errorf("rename temp file %q: %w", cleanedPath, err)
	}

	if err := syncParentDir(dir); err != nil {
		return fmt.Errorf("sync parent directory %q: %w", cleanedPath, err)
	}

	return nil
}
