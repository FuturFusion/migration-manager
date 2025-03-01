package util

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CreateTarballFromDir creates a tarball from a given directory, inclusive of the containing directory.
func CreateTarballFromDir(target string, contentDir string) error {
	f, err := os.Create(target)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	gzipWriter := gzip.NewWriter(f)
	defer func() { _ = gzipWriter.Close() }()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() { _ = tarWriter.Close() }()

	return filepath.Walk(contentDir, archiveDir(tarWriter, contentDir))
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

		err = tarWriter.WriteHeader(&tar.Header{
			Name:    relPath,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		})
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		_, err = io.Copy(tarWriter, f)

		return err
	}
}
