package sys

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	incusUtil "github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal/util"
)

// WorkerImageBuildPrefix is the prefix used for all files that are written as part of the storage volume creation process for the worker image.
const WorkerImageBuildPrefix = "worker-img-build_"

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
	DatabaseDir string // Location of the database files (e.g. /var/lib/migration-manager/database/).
	ACMEDir     string // Location of ACME account files (e.g. /var/cache/migration-manager/acme/).

	ConfigFile string // System config yaml file (e.g. /var/lib/migration-manager/config.yml).
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
		DatabaseDir: util.VarPath("database"),
		ACMEDir:     util.CachePath("acme"),
		ConfigFile:  util.VarPath("config.yml"),
	}

	return newOS
}

func (s *OS) Init() error {
	// Make sure expected directories exist and create them if missing.
	for _, dir := range []string{
		s.CacheDir,
		s.LogDir,
		s.RunDir,
		s.VarDir,
		s.UsrDir,
		s.ShareDir,
		s.ArtifactDir,
		s.ImageDir,
		s.DatabaseDir,
	} {
		if !incusUtil.PathExists(dir) {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetUnixSocket returns the full path to the unix.socket file that this daemon is listening on.
func (s *OS) GetUnixSocket() string {
	path := os.Getenv("MIGRATION_MANAGER_SOCKET")
	if path != "" {
		return path
	}

	return filepath.Join(s.RunDir, "unix.socket")
}

// WorkerImageExists checks if the worker image and binary exist on the filesystem.
func (s *OS) WorkerImageExists(arch string) (string, error) {
	rawWorkerPath := filepath.Join(s.CacheDir, util.RawWorkerImage(arch))
	if util.IsIncusOS() {
		rawWorkerPath = filepath.Join(s.ImageDir, util.RawWorkerImage(arch))
	}

	_, err := os.Stat(rawWorkerPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// If the image doesn't exist yet, then fail.
	if err != nil {
		return "", fmt.Errorf("Missing raw worker image %q: %w", rawWorkerPath, err)
	}

	binaryPath := filepath.Join(s.UsrDir, "migration-manager-worker")
	_, err = os.Stat(binaryPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err != nil {
		return "", fmt.Errorf("Missing worker binary %q: %w", binaryPath, err)
	}

	return rawWorkerPath, nil
}

// LoadWorkerImage writes the VMWare vix tarball to the worker image.
// If the worker image does not exist, it is fetched from the current project version's corresponding GitHub release.
func (s *OS) LoadWorkerImage(ctx context.Context, arch string) (string, error) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	rawWorkerPath, err := s.WorkerImageExists(arch)
	if err != nil {
		return "", err
	}

	// Create a tarball for the worker binary.
	binaryPath := filepath.Join(s.CacheDir, "migration-manager-worker.tar.gz")
	err = util.CreateTarball(ctx, binaryPath, filepath.Join(s.UsrDir, "migration-manager-worker"))
	if err != nil {
		return "", err
	}

	defer func() { _ = os.Remove(binaryPath) }()

	binaryFile, err := os.Open(binaryPath)
	if err != nil {
		return "", err
	}

	rawImgFile, err := os.OpenFile(rawWorkerPath, os.O_RDONLY, 0o600)
	if err != nil {
		return "", err
	}

	defer rawImgFile.Close()

	// Make a copy of the worker image.
	tmpImgPath := filepath.Join(s.CacheDir, WorkerImageBuildPrefix+arch+".img")
	tmpImgFile, err := os.OpenFile(tmpImgPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("Failed to open file %q for writing: %w", tmpImgPath, err)
	}

	defer tmpImgFile.Close()

	// Copy across to the file.
	_, err = io.Copy(tmpImgFile, rawImgFile)
	if err != nil {
		return "", fmt.Errorf("Failed to write file content: %w", err)
	}

	// Move to the first partition offset.
	_, err = tmpImgFile.Seek(616448*512, io.SeekStart)
	if err != nil {
		return "", err
	}

	// Write the migration manager worker at the offset.
	_, err = io.Copy(tmpImgFile, binaryFile)
	if err != nil {
		return "", err
	}

	return tmpImgPath, nil
}
