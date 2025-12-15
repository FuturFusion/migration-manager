package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/revert"
)

// WriteFile reads from the reader and writes to the given file path.
// While the write is in progress, a {filePath).part file will be present.
func (s *OS) WriteFile(filePath string, reader io.ReadCloser) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	reverter := revert.New()
	defer reverter.Fail()

	err := os.MkdirAll(filepath.Dir(filePath), 0o755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", filePath, err)
	}

	partPath := filePath + ".part"

	// Remove any existing part files.
	err = os.Remove(partPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to delete existing stale part file %q: %w", partPath, err)
	}

	// Clear this run's part file on errors.
	reverter.Add(func() { _ = os.Remove(partPath) })

	// Create the part file so we can track progress.
	partFile, err := os.OpenFile(partPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("Failed to open file %q for writing: %w", partPath, err)
	}

	defer partFile.Close()

	// Copy across to the file.
	_, err = io.Copy(partFile, reader)
	if err != nil {
		return fmt.Errorf("Failed to write file content: %w", err)
	}

	err = os.Rename(partPath, filePath)
	if err != nil {
		// Try to remove the target file just in case.
		_ = os.Remove(filePath)

		return fmt.Errorf("Failed to commit file: %w", err)
	}

	reverter.Success()

	return nil
}
