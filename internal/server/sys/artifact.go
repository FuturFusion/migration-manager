package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/revert"
)

// WriteToArtifact reads from the reader and writes to the given file name in the corresponding artifact directory.
// While the write is in progress, a .part file will be present in the directory.
func (s *OS) WriteToArtifact(id uuid.UUID, fileName string, reader io.ReadCloser) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	reverter := revert.New()
	defer reverter.Fail()

	artDir := filepath.Join(s.ArtifactDir, id.String())

	err := os.MkdirAll(artDir, 0o755)
	if err != nil {
		return fmt.Errorf("Failed to create artifact directory %q: %w", artDir, err)
	}

	filePath := filepath.Join(artDir, fileName)
	partPath := filePath + ".part"

	// Remove any existing part files.
	err = os.Remove(partPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to delete existing stale artifact part file %q: %w", partPath, err)
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
