package configcopy

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// File is one config file the repo ships for a tool and where it is deployed.
type File struct {
	Src    string
	Dst    string
	Policy Policy
}

// ToolIDs are the catalog tools that have a config directory in the repo.
// ghostty/ and wezterm/ belong to terminal apps the catalog does not install;
// WSL/ is empty.
var ToolIDs = []string{"zsh", "starship", "zellij"}

// HasConfig reports whether id ships config files.
func HasConfig(id string) bool {
	for _, x := range ToolIDs {
		if x == id {
			return true
		}
	}
	return false
}

// ConfigRoot is $XDG_CONFIG_HOME when it is absolute, else ~/.config.
func ConfigRoot(home, xdg string) string {
	if filepath.IsAbs(xdg) {
		return xdg
	}
	return filepath.Join(home, ".config")
}

// FilesFor lists the files deployed for id. A tool without config returns nil.
//
//	starship  starship/starship.toml (theme) -> $XDG/starship.toml
//	          (falls back to shells/starship/starship.toml)
//	zellij    every file under zellij/ -> $XDG/zellij/ (config, layouts, plugins)
//	zsh       shells/zsh/zshrc -> ~/.zshrc (an existing one is backed up; its
//	          managed blocks are carried over)
func FilesFor(id, repo, home, xdg string) ([]File, error) {
	if repo == "" || home == "" {
		return nil, fmt.Errorf("沒有 repo 或 home")
	}
	cfg := ConfigRoot(home, xdg)
	switch id {
	case "starship":
		src := filepath.Join(repo, "starship", "starship.toml")
		if !isFile(src) {
			src = filepath.Join(repo, "shells", "starship", "starship.toml")
		}
		return []File{{Src: src, Dst: filepath.Join(cfg, "starship.toml"), Policy: PolicyBackupReplace}}, nil
	case "zellij":
		return walkDir(filepath.Join(repo, "zellij"), filepath.Join(cfg, "zellij"))
	case "zsh":
		return []File{{Src: filepath.Join(repo, "shells", "zsh", "zshrc"), Dst: filepath.Join(home, ".zshrc"), Policy: PolicyZshrc}}, nil
	default:
		return nil, nil
	}
}

func walkDir(srcRoot, dstRoot string) ([]File, error) {
	var out []File
	err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		if d.Name() == ".gitkeep" || d.Name() == ".DS_Store" {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		out = append(out, File{Src: path, Dst: filepath.Join(dstRoot, rel), Policy: PolicyBackupReplace})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dst < out[j].Dst })
	return out, nil
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// Undeploy removes a deployed config file.
//
//	absent         nothing at Dst
//	removed        Dst had the repo's bytes and was deleted
//	backed_up      Dst was edited; it was renamed to Dst.bak.<UTC stamp>
//	kept_modified  a copy-if-missing file (~/.zshrc) that the user changed: left alone
//
// Empty parent directories are pruned up to, but not including, the root that
// contains Dst. A symlink at Dst is refused.
func Undeploy(f File, now time.Time, roots ...string) (Result, error) {
	if err := rejectRoots(f.Dst, roots); err != nil {
		return Result{}, err
	}
	info, err := os.Lstat(f.Dst)
	if os.IsNotExist(err) {
		return Result{Action: "absent"}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("拒絕 symlink：%s", f.Dst)
	}
	if !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("不是一般檔案：%s", f.Dst)
	}
	have, err := os.ReadFile(f.Dst)
	if err != nil {
		return Result{}, err
	}
	want, srcErr := os.ReadFile(f.Src)
	same := srcErr == nil && bytes.Equal(have, want)
	if f.Policy == PolicyZshrc && srcErr == nil {
		same = sameRc(have, want)
	}
	var res Result
	switch {
	case same:
		if err := os.Remove(f.Dst); err != nil {
			return Result{}, err
		}
		res.Action = "removed"
	case f.Policy == PolicyCopyIfMissing || f.Policy == PolicyZshrc:
		return Result{Action: "kept_modified"}, nil
	default:
		if err := backup(f.Dst, now); err != nil {
			return Result{}, err
		}
		res.Action = "backed_up"
	}
	pruneEmpty(filepath.Dir(f.Dst), roots)
	return res, nil
}

// pruneEmpty removes empty directories from dir upward, stopping at any root.
func pruneEmpty(dir string, roots []string) {
	stop := map[string]bool{}
	for _, r := range roots {
		if r != "" {
			stop[filepath.Clean(r)] = true
		}
	}
	for dir = filepath.Clean(dir); !stop[dir]; dir = filepath.Dir(dir) {
		inside := false
		for r := range stop {
			if rel, err := filepath.Rel(r, dir); err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !hasDotDot(rel) {
				inside = true
			}
		}
		if !inside {
			return
		}
		if os.Remove(dir) != nil { // fails when not empty
			return
		}
	}
}

func hasDotDot(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}
