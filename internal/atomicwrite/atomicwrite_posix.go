//go:build !windows

package atomicwrite

import (
	"fmt"
	"os"
)

func setTempFileMode(tempFile *os.File, perm os.FileMode) error {
	if err := tempFile.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	return nil
}

func replaceFile(tempPath, targetPath string) error {
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func syncParentDir(path string) error {
	// #nosec G304 -- the directory path is derived from the target file path.
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory %q: %w", path, err)
	}
	defer func() {
		_ = dir.Close()
	}()

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync directory %q: %w", path, err)
	}

	return nil
}
