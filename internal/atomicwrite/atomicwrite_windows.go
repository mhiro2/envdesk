//go:build windows

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

func syncParentDir(string) error {
	return nil
}
