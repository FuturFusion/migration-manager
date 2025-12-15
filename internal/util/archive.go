package util

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// CreateTarball creates a tarball from a given path.
// If the path is a directory, the tarball will include the directory.
// Calls tar -C dirname(contentPath) -czf target basename(contentPath).
func CreateTarball(ctx context.Context, tarballPath string, contentPath string, exclusions ...string) error {
	return createTarball(ctx, nil, io.Discard, tarballPath, contentPath, exclusions...)
}

// CreateTarballWriter creates a tarball from a given path, writing it to the io.Writer.
// If the path is a directory, the tarball will include the directory.
// Calls tar -C dirname(contentPath) -czf - basename(contentPath).
func CreateTarballWriter(ctx context.Context, w io.Writer, contentPath string, exclusions ...string) error {
	return createTarball(ctx, nil, w, "-", contentPath, exclusions...)
}

func createTarball(ctx context.Context, stdin io.Reader, stdout io.Writer, tarballPath string, contentPath string, exclusions ...string) error {
	args := []string{"-C", filepath.Dir(contentPath), "-czf", tarballPath, filepath.Base(contentPath)}

	allArgs := []string{}
	for _, e := range exclusions {
		allArgs = append(allArgs, "--exclude", e)
	}

	allArgs = append(allArgs, args...)
	return subprocess.RunCommandWithFds(ctx, stdin, stdout, "tar", allArgs...)
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
