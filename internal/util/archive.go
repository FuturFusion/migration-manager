package util

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CreateTarball creates a tarball from a given path.
// If the path is a directory, the tarball will include the directory.
func CreateTarball(target string, contentPath string) error {
	f, err := os.Create(target)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	gzipWriter := gzip.NewWriter(f)
	defer func() { _ = gzipWriter.Close() }()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() { _ = tarWriter.Close() }()

	info, err := os.Stat(contentPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(contentPath, archiveDir(tarWriter, contentPath))
	}

	return makeTarball(contentPath, tarWriter, &tar.Header{
		Name:    filepath.Base(contentPath),
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	})
}

func archiveDir(tarWriter *tar.Writer, contentDir string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil || path == contentDir {
			return err
		}

		// Recurse through directories.
		if info.IsDir() {
			return filepath.Walk(path, archiveDir(tarWriter, path))
		}

		relPath, err := filepath.Rel(filepath.Dir(contentDir), path)
		if err != nil {
			return err
		}

		return makeTarball(path, tarWriter, &tar.Header{
			Name:    relPath,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		})
	}
}

func makeTarball(filePath string, tarWriter *tar.Writer, header *tar.Header) error {
	err := tarWriter.WriteHeader(header)
	if err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	_, err = io.Copy(tarWriter, f)

	return err
}
