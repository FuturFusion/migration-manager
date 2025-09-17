package util

import (
	"os"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// CreateTarball creates a tarball from a given path.
// If the path is a directory, the tarball will include the directory.
// Calls tar -C dirname(contentPath) -czf target basename(contentPath).
func CreateTarball(target string, contentPath string) error {
	_, err := subprocess.RunCommand("tar", "-C", filepath.Dir(contentPath), "-czf", target, filepath.Base(contentPath))

	return err
}

// UnpackTarball creates the target directory and calls tar -C <target> -xf  <tarballPath>.
func UnpackTarball(target string, tarballPath string) error {
	err := os.MkdirAll(target, 0o755)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommand("tar", "-C", target, "-xf", tarballPath)
	if err != nil {
		return err
	}

	return nil
}
