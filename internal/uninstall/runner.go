package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/safepath"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

// StatusRemoved is the per-item status of a finished removal.
const StatusRemoved = "removed"

type Runner struct {
	Exec   install.Executor
	Home   string
	XDG    string
	Repo   string
	Bashrc string
	Zshrc  string
	Now    func() time.Time
	// Progress is display only. An empty status means title is starting.
	Progress func(title, status string)
}

// Run executes steps. Once a step of an item fails, the item's later steps are
// skipped, so a package that could not be removed keeps its rc lines and
// config. The exit code is 0 when every item was removed, 1 otherwise.
func (r *Runner) Run(ctx context.Context, steps []Step) ([]install.ItemResult, int) {
	var results []install.ItemResult
	index := map[catalog.ID]int{}
	for _, st := range steps {
		if err := ctx.Err(); err != nil {
			return results, 1
		}
		i, ok := index[st.ItemID]
		if !ok {
			results = append(results, install.ItemResult{ID: st.ItemID, Method: pm.Method(st.Kind), Status: StatusRemoved})
			i = len(results) - 1
			index[st.ItemID] = i
		}
		res := &results[i]
		if res.Status == "failed" {
			r.note(st.Title, "skipped")
			continue
		}
		r.note(st.Title, "")
		note, err := r.step(ctx, st)
		if errors.Is(err, context.Canceled) {
			res.Status = "failed"
			res.ErrText = "已取消"
			return results, 1
		}
		if len(st.Argv) > 0 && res.Argv == nil {
			res.Argv = append([]string(nil), st.Argv...)
		}
		if err != nil {
			res.Status = "failed"
			res.ErrText = err.Error()
			r.note(st.Title, "failed")
			continue
		}
		if note != "" {
			if res.Note != "" {
				res.Note += "；"
			}
			res.Note += note
		}
		r.note(st.Title, "ok")
	}
	code := 0
	for _, res := range results {
		if res.Status != StatusRemoved {
			code = 1
		}
	}
	return results, code
}

func (r *Runner) note(title, status string) {
	if r.Progress != nil {
		r.Progress(title, status)
	}
}

func (r *Runner) step(ctx context.Context, st Step) (string, error) {
	switch st.Kind {
	case KindPackage:
		if err := pm.ValidateArgv(st.Argv); err != nil {
			return "", err
		}
		if err := r.exec(ctx, st.Argv, st.Manager); err != nil {
			return "", err
		}
		return "", r.dropAliasLinks(st.Links)
	case KindRustup:
		if len(st.Argv) != 4 || filepath.Base(st.Argv[0]) != "rustup" || !filepath.IsAbs(st.Argv[0]) || strings.Join(st.Argv[1:], " ") != "self uninstall -y" {
			return "", fmt.Errorf("不允許的 rustup argv")
		}
		return "", r.exec(ctx, st.Argv, "")
	case KindGo:
		return "", r.removeGo(ctx, st)
	case KindRelease:
		return "", r.removeRelease(st)
	case KindShell:
		return r.removeBlocks(st.Blocks)
	case KindConfig:
		return r.removeConfig(st.ItemID)
	default:
		return "", fmt.Errorf("未知的步驟 %s", st.Kind)
	}
}

func (r *Runner) exec(ctx context.Context, argv []string, manager string) error {
	if r.Exec == nil {
		return fmt.Errorf("沒有 Executor")
	}
	out, code, err := r.Exec.Run(ctx, argv)
	cls := install.ClassExec(manager, out, code, err)
	if cls.Canceled {
		return context.Canceled
	}
	if cls.ExecErr != nil {
		return cls.ExecErr
	}
	if cls.ErrText != "" {
		return errors.New(cls.ErrText)
	}
	if cls.ExitCode != 0 {
		return fmt.Errorf("結束碼 %d", cls.ExitCode)
	}
	return nil
}

// dropAliasLinks removes the Debian fd/bat symlinks the installer created.
// Only a symlink that points at /usr/bin/fdfind or /usr/bin/batcat is removed.
func (r *Runner) dropAliasLinks(links []string) error {
	for _, l := range links {
		info, err := os.Lstat(l)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(l)
		if err != nil {
			continue
		}
		if target != "/usr/bin/fdfind" && target != "/usr/bin/batcat" {
			continue
		}
		if err := os.Remove(l); err != nil {
			return err
		}
	}
	return nil
}

func validGoDir(dir string) bool {
	return filepath.IsAbs(dir) && filepath.Base(dir) == "go" && filepath.Clean(dir) == dir && dir != "/go"
}

func (r *Runner) removeGo(ctx context.Context, st Step) error {
	if !validGoDir(st.GoDir) {
		return fmt.Errorf("拒絕刪除 %s", st.GoDir)
	}
	if len(st.Argv) > 0 {
		want := []string{"sudo", "rm", "-rf", st.GoDir}
		if strings.Join(st.Argv, "\x00") != strings.Join(want, "\x00") {
			return fmt.Errorf("不允許的 go 移除 argv")
		}
		return r.exec(ctx, st.Argv, "")
	}
	if _, err := os.Stat(filepath.Join(st.GoDir, "bin", "go")); err != nil {
		return fmt.Errorf("%s 不是 Go 安裝目錄", st.GoDir)
	}
	return os.RemoveAll(st.GoDir)
}

// removeRelease deletes ~/.local/bin/<bin>. When it is a symlink into
// ~/.local/share/<sub>/<top>/..., that unpacked <top> tree goes too, and
// ~/.local/share/<sub> is removed only if nothing else is left in it (for
// neovim that directory also holds the user's own data).
func (r *Runner) removeRelease(st Step) error {
	if r.Home == "" {
		return fmt.Errorf("沒有 home")
	}
	binDir := filepath.Join(r.Home, ".local", "bin")
	if filepath.Dir(st.Bin) != binDir {
		return fmt.Errorf("拒絕刪除 %s", st.Bin)
	}
	if err := safepath.RejectEscapingSymlink(binDir, r.Home); err != nil {
		return err
	}
	info, err := os.Lstat(st.Bin)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var tree string
	if info.Mode()&os.ModeSymlink != 0 && st.ShareDir != "" {
		target, err := os.Readlink(st.Bin)
		if err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(binDir, target)
			}
			tree = treeRoot(st.ShareDir, filepath.Clean(target))
		}
	}
	if err := os.Remove(st.Bin); err != nil {
		return err
	}
	if tree == "" {
		return nil
	}
	shareRoot := filepath.Join(r.Home, ".local", "share")
	if filepath.Dir(st.ShareDir) != shareRoot {
		return fmt.Errorf("拒絕刪除 %s", st.ShareDir)
	}
	if err := safepath.RejectEscapingSymlink(tree, r.Home); err != nil {
		return err
	}
	if err := os.RemoveAll(tree); err != nil {
		return err
	}
	_ = os.Remove(st.ShareDir) // only succeeds when empty
	return nil
}

// treeRoot is share/<top> when target lies under share/<top>/..., else "".
func treeRoot(share, target string) string {
	rel, err := filepath.Rel(share, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	top := strings.Split(rel, string(filepath.Separator))[0]
	if top == "" || top == "." || top == ".." {
		return ""
	}
	return filepath.Join(share, top)
}

func (r *Runner) rcPath(sh shellrc.Shell) string {
	switch {
	case sh == shellrc.Bash && r.Bashrc != "":
		return r.Bashrc
	case sh == shellrc.Zsh && r.Zshrc != "":
		return r.Zshrc
	}
	return shellrc.RcPath(r.Home, sh)
}

func (r *Runner) removeBlocks(blocks []shellrc.BlockID) (string, error) {
	if r.Home == "" {
		return "", fmt.Errorf("沒有 home")
	}
	var touched []string
	for _, sh := range shellrc.Shells {
		path := r.rcPath(sh)
		removed, err := shellrc.RemoveFile(path, r.Home, blocks)
		if err != nil {
			return "", err
		}
		if len(removed) > 0 {
			touched = append(touched, "~/"+filepath.Base(path))
		}
	}
	if len(touched) == 0 {
		return "", nil
	}
	return "已清除 " + strings.Join(touched, "、") + " 的設定區塊", nil
}

func (r *Runner) removeConfig(id catalog.ID) (string, error) {
	if r.Repo == "" || r.Home == "" {
		return "找不到 repo，設定檔未處理", nil
	}
	files, err := configcopy.FilesFor(string(id), r.Repo, r.Home, r.XDG)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	roots := []string{r.Home, configcopy.ConfigRoot(r.Home, r.XDG)}
	var notes []string
	for _, f := range files {
		res, err := configcopy.Undeploy(f, now, roots...)
		if err != nil {
			return "", err
		}
		switch res.Action {
		case "backed_up":
			notes = append(notes, "已備份修改過的 "+r.short(f.Dst))
		case "kept_modified":
			notes = append(notes, "保留你修改過的 "+r.short(f.Dst))
		}
	}
	return strings.Join(notes, "；"), nil
}

func (r *Runner) short(p string) string {
	if rel, err := filepath.Rel(r.Home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}
