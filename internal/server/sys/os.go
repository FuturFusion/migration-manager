package sys

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// OS is a high-level facade for accessing operating-system level functionalities.
type OS struct {
	// A lock to manage filesystem access during uploads.
	uploadLock sync.Mutex

	// Directories
	CacheDir string // Cache directory (e.g., /var/cache/migration-manager/)
	LogDir   string // Log directory (e.g. /var/log/).
	RunDir   string // Runtime directory (e.g. /run/migration-manager/).
	VarDir   string // Data directory (e.g. /var/lib/migration-manager/).
	ShareDir string // Static directory (e.g. /usr/share/migration-manager/).
	UsrDir   string // Static directory (e.g. /usr/lib/migration-manager/).
}

// DefaultOS returns a fresh uninitialized OS instance with default values.
func DefaultOS() *OS {
	newOS := &OS{
		CacheDir: util.CachePath(),
		LogDir:   util.LogPath(),
		RunDir:   util.RunPath(),
		VarDir:   util.VarPath(),
		UsrDir:   util.UsrPath(),
		ShareDir: util.SharePath(),
	}

	return newOS
}

// DefaultSDKPath returns the default paths for a source's sdk file, and its partial import file.
func (s *OS) DefaultSDKPath(srcType api.SourceType) (sdkName string, sdkPartName string) {
	sdkName = filepath.Join(s.VarDir, string(srcType), string(srcType)+".sdk")
	sdkPartName = sdkName + ".part"

	return sdkName, sdkPartName
}

// CleanPartialSDKs removes partial SDK uploads.
func (s *OS) CleanPartialSDKs() error {
	s.uploadLock.Lock()
	defer s.uploadLock.Unlock()

	for _, srcType := range api.VMSourceTypes() {
		filePath, partPath := s.DefaultSDKPath(srcType)

		_, err := os.Stat(partPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Failed to inspect part file for %q: %w", filePath, err)
		}

		if err == nil {
			err := os.Remove(filePath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("Failed to remove incomplete sdk file %q: %w", filePath, err)
			}

			err = os.Remove(partPath)
			if err != nil {
				return fmt.Errorf("Failed to remove sdk part file for %q: %w", filePath, err)
			}
		}
	}

	return nil
}

// WriteSDK reads from the reader and writes to the default SDK path for the given source.
// While the write is in progress, a .part file will be present.
func (s *OS) WriteSDK(srcType api.SourceType, reader io.ReadCloser) error {
	s.uploadLock.Lock()
	defer s.uploadLock.Unlock()

	filePath, partPath := s.DefaultSDKPath(srcType)

	reverter := revert.New()
	defer reverter.Fail()

	// Create the directory if it doesn't exist yet.
	err := os.MkdirAll(filepath.Dir(partPath), 0o755)
	if err != nil {
		return fmt.Errorf("Failed to create SDK source %q directory: %w", srcType, err)
	}

	// Remove any existing part files.
	err = os.Remove(partPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to delete existing SDK file for %q", srcType)
	}

	// Clear this run on errors.
	reverter.Add(func() {
		_ = os.Remove(partPath)
	})

	// Create a part file so we can track progress.
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
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
		// Target file may be corrupted, so remove it.
		_ = os.Remove(filePath)

		return fmt.Errorf("Failed to commit file: %w", err)
	}

	reverter.Success()

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

// LocalDatabaseDir returns the path of the local database directory.
func (s *OS) LocalDatabaseDir() string {
	return filepath.Join(s.VarDir, "database")
}

// ValidateFileSystem returns whether the required and optional files have been supplied to Migration Manager.
func (s *OS) ValidateFileSystem(sources ...api.SourceType) error {
	s.uploadLock.Lock()
	defer s.uploadLock.Unlock()

	for _, src := range sources {
		_, err := s.GetSDKName(api.SOURCETYPE_VMWARE)
		if err != nil {
			return fmt.Errorf("Failed to find SDK for source type %q: %w", src, err)
		}
	}

	// Ensure exactly zero or one VirtIO drivers ISOs exist.
	_, err := s.GetVirtioDriversISOName()
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

// GetSDKName returns the file name for a default source sdk. Returns 404 if none exist.
// If a .part file exists, it returns an error because the sdk file is likely incomplete.
func (s *OS) GetSDKName(srcType api.SourceType) (string, error) {
	filePath, partPath := s.DefaultSDKPath(api.SOURCETYPE_VMWARE)
	_, err := os.Stat(partPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("Failed to search for in-progress SDK upload file %q: %w", partPath, err)
	}

	if err == nil {
		return "", fmt.Errorf("SDK import in progress")
	}

	info, err := os.Stat(filePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("Failed to inspect sdk file %q: %w", filePath, err)
	}

	// Return 404 if not found.
	if err != nil {
		return "", incusAPI.StatusErrorf(http.StatusNotFound, "SDK file %q not found", filePath)
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Unexpected file mode %q", info.Mode().String())
	}

	return filepath.Base(filePath), nil
}

// LoadWorkerImage writes the VMWare vix tarball to the worker image.
// If the worker image does not exist, it is fetched from the current project version's corresponding GitHub release.
func (s *OS) LoadWorkerImage(ctx context.Context, srcType api.SourceType) error {
	s.uploadLock.Lock()
	defer s.uploadLock.Unlock()

	sdkName, err := s.GetSDKName(srcType)
	if err != nil {
		return err
	}

	// Create a tarball for the worker binary.
	binaryPath := filepath.Join(s.CacheDir, "migration-manager-worker.tar.gz")
	err = util.CreateTarball(binaryPath, filepath.Join(s.UsrDir, "migration-manager-worker"))
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

	sdkFile, err := os.Open(filepath.Join(s.VarDir, string(srcType), sdkName))
	if err != nil {
		return err
	}

	defer sdkFile.Close()

	// Move to the first partition offset.
	_, err = rawImgFile.Seek(616448*512, io.SeekStart)
	if err != nil {
		return err
	}

	// Write the VIX tarball at the offset.
	_, err = io.Copy(rawImgFile, sdkFile)
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
