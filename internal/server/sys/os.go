package sys

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/FuturFusion/migration-manager/internal/util"
)

// OS is a high-level facade for accessing operating-system level functionalities.
type OS struct {
	// Directories
	CacheDir string // Cache directory (e.g., /var/cache/migration-manager/)
	LogDir   string // Log directory (e.g. /var/log/).
	RunDir   string // Runtime directory (e.g. /run/migration-manager/).
	VarDir   string // Data directory (e.g. /var/lib/migration-manager/).
}

// DefaultOS returns a fresh uninitialized OS instance with default values.
func DefaultOS() *OS {
	newOS := &OS{
		CacheDir: util.CachePath(),
		LogDir:   util.LogPath(),
		RunDir:   util.RunPath(),
		VarDir:   util.VarPath(),
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

// ValidateFileSystem returns whether the required and optional files have been supplied to Migration Manager.
func (s *OS) ValidateFileSystem() error {
	_, err := s.GetVMwareVixName()
	if err != nil {
		return fmt.Errorf("Failed to find VMWare vix tarball: %w", err)
	}

	// Ensure exactly zero or one VirtIO drivers ISOs exist.
	_, err = s.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to find Virtio drivers ISO: %w", err)
	}

	return nil
}

// GetVirtioDriversISOName returns the name of the virtio drivers ISO image.
func (s *OS) GetVirtioDriversISOName() (string, error) {
	files, err := filepath.Glob(fmt.Sprintf("%s/virtio-win-*.iso", s.CacheDir))
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", os.ErrNotExist
	}

	if len(files) != 1 {
		return "", fmt.Errorf("Unable to determine virtio drivers ISO name")
	}

	return filepath.Base(files[0]), nil
}

// GetVMwareVixName returns the name of the VMWare vix disklib tarball.
func (s *OS) GetVMwareVixName() (string, error) {
	files, err := filepath.Glob(filepath.Join(s.VarDir, "VMware-vix-disklib*.tar.gz"))
	if err != nil {
		return "", fmt.Errorf("Failed to find VMware vix tarball in %q: %w", s.VarDir, err)
	}

	if len(files) == 0 {
		return "", os.ErrNotExist
	}

	if len(files) != 1 {
		return "", fmt.Errorf("Failed to find exactly one VMWare vix tarball in %q (Found %d)", s.VarDir, len(files))
	}

	return filepath.Base(files[0]), nil
}

// LoadWorkerImage writes the VMWare vix tarball to the worker image.
// If the worker image does not exist, it is fetched from the current project version's corresponding GitHub release.
func (s *OS) LoadWorkerImage(ctx context.Context) error {
	vixName, err := s.GetVMwareVixName()
	if err != nil {
		return err
	}

	// Create a tarball for the worker binary.
	binaryPath := filepath.Join(s.CacheDir, "migration-manager-worker.tar.gz")
	err = util.CreateTarball(binaryPath, filepath.Join(s.VarDir, "migration-manager-worker"))
	if err != nil {
		return err
	}

	defer func() { _ = os.Remove(binaryPath) }()

	binaryFile, err := os.Open(binaryPath)
	if err != nil {
		return err
	}

	rawWorkerPath := filepath.Join(s.CacheDir, util.RawWorkerImage())
	_, err = os.Stat(rawWorkerPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the image doesn't exist yet, then download it from GitHub.
	if err != nil {
		g, err := util.GetProjectRepo(ctx, false)
		if err != nil {
			return err
		}

		err = g.DownloadAsset(ctx, rawWorkerPath, "migration-manager-worker.img.gz")
		if err != nil {
			return err
		}
	}

	rawImgFile, err := os.OpenFile(rawWorkerPath, os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	defer rawImgFile.Close()

	vixFile, err := os.Open(filepath.Join(s.VarDir, vixName))
	if err != nil {
		return err
	}

	defer vixFile.Close()

	// Move to the first partition offset.
	_, err = rawImgFile.Seek(616448*512, io.SeekStart)
	if err != nil {
		return err
	}

	// Write the VIX tarball at the offset.
	_, err = io.Copy(rawImgFile, vixFile)
	if err != nil {
		return err
	}

	// Move to the next partition offset.
	_, err = rawImgFile.Seek(821248*512, io.SeekStart)
	if err != nil {
		return err
	}

	// Write the migration manager worker at the offset.
	_, err = io.Copy(rawImgFile, binaryFile)
	if err != nil {
		return err
	}

	return nil
}

// LoadVirtioWinISO attempts to fetch the latest virtio-win ISO, returning the path to the file.
func (s *OS) LoadVirtioWinISO() (string, error) {
	iso, err := s.GetVirtioDriversISOName()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err == nil {
		return filepath.Join(s.CacheDir, iso), nil
	}

	resp, err := http.Get("https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso")
	if err != nil {
		return "", fmt.Errorf("Failed to fetch latest virtio-win ISO: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	versionedName := filepath.Base(resp.Request.URL.Path)
	if !strings.HasPrefix(versionedName, "virtio-win-") || !strings.HasSuffix(versionedName, ".iso") {
		return "", fmt.Errorf("VirtIO drivers ISO is not available. Found artifact: %q", versionedName)
	}

	isoPath := filepath.Join(s.CacheDir, versionedName)
	isoFile, err := os.Create(isoPath)
	if err != nil {
		return "", err
	}

	defer func() { _ = isoFile.Close() }()

	_, err = io.Copy(isoFile, resp.Body)
	if err != nil {
		return "", err
	}

	return isoPath, nil
}
