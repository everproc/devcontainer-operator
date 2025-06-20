package features

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TODO(juf): add helper to read this from .dockerignore and or .gitignore
var ignorePaths = []string{".git"}

func tarGzFromWorkspaceAndCacheWithFile(
	workspacePath string,
	cachePath string,
	fileContent []byte,
	fileName string,
	out io.Writer,
) error {
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	defer MustClose(tw, gz)

	hdr := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(fileContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(fileContent); err != nil {
		return err
	}
	err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, p := range ignorePaths {
			if strings.HasPrefix(path, p) {
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(cachePath, path)
		if err != nil {
			return err
		}
		// Avoid name collision with injected file
		if relPath == fileName {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer MustClose(f)
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}

	return filepath.WalkDir(cachePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, p := range ignorePaths {
			if strings.HasPrefix(path, p) {
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(cachePath, path)
		if err != nil {
			return err
		}
		// Avoid name collision with injected file
		if relPath == fileName {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer MustClose(f)
		_, err = io.Copy(tw, f)
		return err
	})
}
