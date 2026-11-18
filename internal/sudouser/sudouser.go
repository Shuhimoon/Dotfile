// Package sudouser handles `sudo ./tools`: the process runs as root, but the
// tools, configs and shell lines belong to the user who typed sudo. Without
// this, HOME is /root and everything user-scoped (~/.local/bin, ~/.config,
// ~/.bashrc, ~/.zshrc, ~/.ssh, ~/.cargo) lands in root's home.
package sudouser

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Target is the real user behind sudo.
type Target struct {
	Name string
	Home string
	UID  int
	GID  int
}

// Detect returns the invoking user when euid is 0 and SUDO_USER names a
// non-root account. ok is false for a normal run and for a real root login
// (no SUDO_USER, or SUDO_USER=root): root's own home is right then.
func Detect(euid int, getenv func(string) string, lookup func(string) (*user.User, error)) (t Target, ok bool, err error) {
	if euid != 0 {
		return Target{}, false, nil
	}
	name := strings.TrimSpace(getenv("SUDO_USER"))
	if name == "" || name == "root" {
		return Target{}, false, nil
	}
	if lookup == nil {
		lookup = user.Lookup
	}
	u, err := lookup(name)
	if err != nil {
		return Target{}, false, fmt.Errorf("找不到 sudo 使用者 %s：%w", name, err)
	}
	uid, err := pickID(getenv("SUDO_UID"), u.Uid)
	if err != nil {
		return Target{}, false, fmt.Errorf("sudo 使用者 %s 的 uid：%w", name, err)
	}
	gid, err := pickID(getenv("SUDO_GID"), u.Gid)
	if err != nil {
		return Target{}, false, fmt.Errorf("sudo 使用者 %s 的 gid：%w", name, err)
	}
	home := filepath.Clean(u.HomeDir)
	if uid == 0 || !filepath.IsAbs(home) || home == "/" {
		return Target{}, false, fmt.Errorf("sudo 使用者 %s 的家目錄不可用：%q", name, u.HomeDir)
	}
	return Target{Name: name, Home: home, UID: uid, GID: gid}, true, nil
}

func pickID(env, fallback string) (int, error) {
	s := strings.TrimSpace(env)
	if s == "" {
		s = fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("不是數字：%q", s)
	}
	return n, nil
}

// Env points HOME at the user's home for this process and its children
// (os.UserHomeDir, rustup-init). XDG_CONFIG_HOME is dropped when it points
// outside that home (a value kept from root's environment).
func (t Target) Env(setenv func(k, v string) error, getenv func(string) string, unsetenv func(string) error) error {
	if err := setenv("HOME", t.Home); err != nil {
		return err
	}
	if x := getenv("XDG_CONFIG_HOME"); x != "" && !within(x, t.Home) {
		return unsetenv("XDG_CONFIG_HOME")
	}
	return nil
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Roots are the user-scoped paths tools writes under home.
func Roots(home string) []string {
	roots := []string{".local", ".config", ".cargo", ".rustup", ".ssh", ".zsh"}
	out := make([]string, 0, len(roots)+4)
	for _, r := range roots {
		out = append(out, filepath.Join(home, r))
	}
	for _, pat := range []string{".bashrc*", ".zshrc*"} {
		m, _ := filepath.Glob(filepath.Join(home, pat))
		out = append(out, m...)
	}
	return out
}

// Fixer gives root-owned files under the roots back to the user. Files that
// already belong to someone else are left alone, symlinks are changed
// themselves (lchown) and never followed.
type Fixer struct {
	UID, GID int
	// Owner reports a file's uid; ok is false when unknown.
	Owner func(fs.FileInfo) (uid int, ok bool)
	// Lchown changes the owner of path without following a symlink.
	Lchown func(path string, uid, gid int) error
}

// NewFixer is a Fixer for t with the platform's owner lookup and os.Lchown.
func NewFixer(t Target) Fixer {
	return Fixer{UID: t.UID, GID: t.GID, Owner: fileOwner, Lchown: os.Lchown}
}

// Fix walks roots (missing ones are skipped) and returns how many entries
// changed owner.
func (f Fixer) Fix(roots []string) (int, error) {
	if f.Owner == nil || f.Lchown == nil {
		return 0, nil
	}
	n := 0
	var firstErr error
	for _, root := range roots {
		if _, err := os.Lstat(root); err != nil {
			continue
		}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			uid, ok := f.Owner(info)
			if !ok || uid != 0 {
				return nil
			}
			if err := f.Lchown(p, f.UID, f.GID); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return nil
			}
			n++
			return nil
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return n, firstErr
}
