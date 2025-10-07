package sys

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/FuturFusion/migration-manager/internal/util"
)

// OS is a high-level facade for accessing operating-system level functionalities.
type OS struct {
	// A lock to manage filesystem access during writes.
	writeLock sync.Mutex

	// Directories
	CacheDir string // Cache directory (e.g., /var/cache/migration-manager/)
	LogDir   string // Log directory (e.g. /var/log/).
	RunDir   string // Runtime directory (e.g. /run/migration-manager/).
	VarDir   string // Data directory (e.g. /var/lib/migration-manager/).
	ShareDir string // Static directory (e.g. /usr/share/migration-manager/).
	UsrDir   string // Static directory (e.g. /usr/lib/migration-manager/).

	ArtifactDir string // Location of user-supplied files (e.g. /var/lib/migration-manager/artifacts/).
	ImageDir    string // Location of the worker images (e.g. /usr/share/migration-manager/images/).
}

// DefaultOS returns a fresh uninitialized OS instance with default values.
func DefaultOS() *OS {
	newOS := &OS{
		CacheDir:    util.CachePath(),
		LogDir:      util.LogPath(),
		RunDir:      util.RunPath(),
		VarDir:      util.VarPath(),
		UsrDir:      util.UsrPath(),
		ShareDir:    util.SharePath(),
		ArtifactDir: util.VarPath("artifacts"),
		ImageDir:    util.SharePath("images"),
	}

	return newOS
}

// GetUnixSocket returns the full path to the unix.socket file that this daemon is listening on.
func (s *OS) GetUnixSocket() string {
	path := os.Getenv("MIGRATION_MANAGER_SOCKET")
	if path != "" {
		return path
	}

	return filepath.Join(s.RunDir, "unix.socket")
}

// LocalDatabaseDir returns the path of the local database directory.
func (s *OS) LocalDatabaseDir() string {
	return filepath.Join(s.VarDir, "database")
}

// LoadWorkerImage writes the VMWare vix tarball to the worker image.
// If the worker image does not exist, it is fetched from the current project version's corresponding GitHub release.
func (s *OS) LoadWorkerImage(ctx context.Context, arch string) (string, error) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	// Create a tarball for the worker binary.
	binaryPath := filepath.Join(s.CacheDir, "migration-manager-worker.tar.gz")
	err := util.CreateTarball(binaryPath, filepath.Join(s.UsrDir, "migration-manager-worker"))
	if err != nil {
		return "", err
	}

	defer func() { _ = os.Remove(binaryPath) }()

	binaryFile, err := os.Open(binaryPath)
	if err != nil {
		return "", err
	}

	rawWorkerPath := filepath.Join(s.CacheDir, util.RawWorkerImage(arch))
	if util.IsIncusOS() {
		rawWorkerPath = filepath.Join(s.ImageDir, util.RawWorkerImage(arch))
	}

	_, err = os.Stat(rawWorkerPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// If the image doesn't exist yet, then download it from GitHub.
	if err != nil {
		g, err := util.GetProjectRepo(ctx, false)
		if err != nil {
			return "", err
		}

		err = g.DownloadAsset(ctx, rawWorkerPath, "migration-manager-worker.img.gz")
		if err != nil {
			return "", err
		}
	}

	rawImgFile, err := os.OpenFile(rawWorkerPath, os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}

	defer rawImgFile.Close()

	// Move to the first partition offset.
	_, err = rawImgFile.Seek(616448*512, io.SeekStart)
	if err != nil {
		return "", err
	}

	// Write the migration manager worker at the offset.
	_, err = io.Copy(rawImgFile, binaryFile)
	if err != nil {
		return "", err
	}

	return rawWorkerPath, nil
}
