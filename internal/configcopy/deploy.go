package configcopy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shuhi/dotfiles/internal/safepath"
)

type Policy int

const (
	PolicyBackupReplace Policy = iota
	PolicyCopyIfMissing
	// PolicyZshrc deploys the ~/.zshrc template: an existing file is backed up
	// and replaced, and the managed dotfiles blocks it had (fzf…) are carried
	// over. A file that is already the template plus managed blocks is left
	// alone. Undeploy keeps a ~/.zshrc the user edited.
	PolicyZshrc
	// PolicyZellijConfig is PolicyBackupReplace for zellij/config.kdl, with the
	// zjstatus.wasm path rewritten to the deployed plugin's absolute path.
	PolicyZellijConfig
)

type Result struct {
	Action string // created, unchanged, backed_up, skipped_exists
}

// AllowedDest is true when dst stays inside home or, when xdg is absolute, inside xdg.
// Either root passing is enough. Relative xdg is ignored.
func AllowedDest(dst, home, xdg string) bool {
	if home != "" && safepath.RejectEscapingSymlink(dst, home) == nil {
		return true
	}
	if xdg != "" && filepath.IsAbs(xdg) && safepath.RejectEscapingSymlink(dst, xdg) == nil {
		return true
	}
	return false
}

// Deploy copies src to dst. Same bytes are left untouched, including mtime.
// roots are the directories the caller already accepted. An empty list does not
// fall back to the destination's parent: a missing intermediate directory under
// an escaping symlink would otherwise be created outside the root.
func Deploy(src, dst string, policy Policy, now time.Time, roots ...string) (Result, error) {
	if err := rejectRoots(dst, roots); err != nil {
		return Result{}, err
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Lstat(dst)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	if exists && info.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("拒絕 symlink：%s", dst)
	}
	if policy == PolicyCopyIfMissing && exists {
		return Result{Action: "skipped_exists"}, nil
	}
	if policy == PolicyZshrc {
		return deployZshrc(dst, body, exists, now)
	}
	if policy == PolicyZellijConfig {
		body = zellijConfig(dst, body)
	}
	if exists {
		old, err := os.ReadFile(dst)
		if err != nil {
			return Result{}, err
		}
		if bytes.Equal(old, body) {
			return Result{Action: "unchanged"}, nil
		}
		if err := backup(dst, now); err != nil {
			return Result{}, err
		}
		if err := writeAtomic(dst, body, 0o644); err != nil {
			return Result{}, err
		}
		return Result{Action: "backed_up"}, nil
	}
	if err := writeAtomic(dst, body, 0o644); err != nil {
		return Result{}, err
	}
	return Result{Action: "created"}, nil
}

func rejectRoots(dst string, roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("沒有允許的 root")
	}
	var last error
	for _, root := range roots {
		if root == "" {
			last = fmt.Errorf("沒有允許的 root")
			continue
		}
		if err := safepath.RejectEscapingSymlink(dst, root); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		return fmt.Errorf("沒有允許的 root")
	}
	return last
}

func backup(dst string, now time.Time) error {
	stamp := now.UTC().Format("20060102T150405Z")
	base := dst + ".bak." + stamp
	name := base
	for i := 0; ; i++ {
		if i > 0 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		_, err := os.Lstat(name)
		if os.IsNotExist(err) {
			return os.Rename(dst, name)
		}
		if err != nil {
			return err
		}
		if i > 1000 {
			return fmt.Errorf("太多備份：%s", dst)
		}
	}
}

func writeAtomic(dst string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".cfg-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	ok = true
	return nil
}
