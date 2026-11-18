package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KindZshPlugins clones the zsh plugins the ~/.zshrc template sources.
const KindZshPlugins Kind = "zsh-plugins"

// ZshPlugin is one plugin cloned into ~/.zsh/<Name>.
type ZshPlugin struct {
	Name string
	URL  string
}

// ZshPlugins are sourced by shells/zsh/zshrc (each guarded by a file check).
var ZshPlugins = []ZshPlugin{
	{Name: "fzf-tab", URL: "https://github.com/Aloxaf/fzf-tab"},
	{Name: "zsh-autosuggestions", URL: "https://github.com/zsh-users/zsh-autosuggestions"},
	{Name: "zsh-syntax-highlighting", URL: "https://github.com/zsh-users/zsh-syntax-highlighting"},
}

// ZshDir is ~/.zsh.
func ZshDir(home string) string { return filepath.Join(home, ".zsh") }

// ZshPluginArgv is the clone command for p.
func ZshPluginArgv(home string, p ZshPlugin) []string {
	return []string{"git", "clone", "--depth", "1", p.URL, filepath.Join(ZshDir(home), p.Name)}
}

// doZshPlugins creates ~/.zsh/cache and clones each missing plugin. A plugin
// directory that already exists is kept as it is (idempotent). It runs only
// when zsh itself was installed or already there.
func (r *Runner) doZshPlugins(ctx context.Context, results []ItemResult) (ItemResult, string, bool) {
	cur, ok := findResult(results, "zsh")
	if !ok || !successStatus(cur.Status) {
		return cur, "skipped", false
	}
	if r.Home == "" {
		return cur, "skipped", false
	}
	dir := ZshDir(r.Home)
	if err := os.MkdirAll(filepath.Join(dir, "cache"), 0o755); err != nil {
		cur.Note = joinNote(cur.Note, "zsh 外掛："+err.Error())
		return cur, "failed", false
	}
	var failed []string
	cloned := 0
	for _, p := range ZshPlugins {
		if _, err := os.Lstat(filepath.Join(dir, p.Name)); err == nil {
			continue
		}
		if r.Exec == nil {
			failed = append(failed, p.Name)
			continue
		}
		out, code, err := r.Exec.Run(ctx, ZshPluginArgv(r.Home, p))
		cls := ClassExec("", out, code, err)
		if cls.Canceled {
			return cur, "", true
		}
		if cls.ExecErr != nil || cls.ErrText != "" {
			failed = append(failed, p.Name)
			continue
		}
		cloned++
	}
	if len(failed) > 0 {
		cur.Note = joinNote(cur.Note, fmt.Sprintf("zsh 外掛下載失敗：%s（需要 git 與網路）", strings.Join(failed, "、")))
		return cur, "failed", false
	}
	if cloned > 0 {
		cur.Note = joinNote(cur.Note, fmt.Sprintf("已下載 %d 個 zsh 外掛到 ~/.zsh", cloned))
	}
	return cur, "ok", false
}
