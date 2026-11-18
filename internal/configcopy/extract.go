package configcopy

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Extract copies every regular file of fsys into dir with the same layout.
// A file whose content already matches is left alone, so running it on each
// start is cheap and an older extraction is brought up to date.
func Extract(fsys fs.FS, dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("extract: %s 不是絕對路徑", dir)
	}
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !fs.ValidPath(path) {
			return fmt.Errorf("extract: 不合法的路徑 %s", path)
		}
		dst := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, data) {
			return nil
		}
		tmp, err := os.CreateTemp(filepath.Dir(dst), ".extract-*")
		if err != nil {
			return err
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return err
		}
		if err := os.Chmod(tmp.Name(), 0o644); err != nil {
			os.Remove(tmp.Name())
			return err
		}
		return os.Rename(tmp.Name(), dst)
	})
}
