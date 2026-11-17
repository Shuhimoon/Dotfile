package safepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RejectEscapingSymlink rejects path when it escapes root or when path
// or an existing ancestor through root is a symlink. Missing trailing
// components are allowed so a not-yet-created file can still be checked.
// It does not call EvalSymlinks.
func RejectEscapingSymlink(path, root string) error {
	if path == "" || root == "" {
		return fmt.Errorf("路徑或 root 為空")
	}
	if strings.ContainsRune(path, '\n') || strings.ContainsRune(path, 0) {
		return fmt.Errorf("拒絕路徑：含有換行或 NUL")
	}
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) || !filepath.IsAbs(cleanRoot) {
		return fmt.Errorf("路徑必須是絕對路徑")
	}
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("路徑在 root 之外：%s", cleanPath)
	}
	cur := cleanPath
	for {
		info, err := os.Lstat(cur)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("拒絕 symlink：%s", cur)
		}
		if cur == cleanRoot {
			return nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil
		}
		cur = parent
	}
}
