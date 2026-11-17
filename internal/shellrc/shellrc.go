// Package shellrc writes and removes the per-tool managed blocks in ~/.bashrc
// and ~/.zshrc. Every block is fenced by
//
//	# >>> dotfiles:<id> >>>
//	...
//	# <<< dotfiles:<id> <<<
//
// so uninstall removes exactly the lines this tool added. Both files get the
// same blocks, written in each shell's own syntax, whatever the login shell is.
package shellrc

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/safepath"
	"github.com/shuhi/dotfiles/shells/bash/hooks"
)

type Shell string

const (
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
)

// Shells is every rc file the installer manages, in write order.
var Shells = []Shell{Bash, Zsh}

type BlockID string

const (
	BlockLocalBin BlockID = "local-bin"
	BlockStarship BlockID = "starship"
	BlockFzf      BlockID = "fzf"
	BlockRust     BlockID = "rust"
	BlockGo       BlockID = "go"
)

// canonical is the order blocks appear in a file. local-bin comes first so a
// release binary in ~/.local/bin is on PATH before starship/fzf look for it.
var canonical = []BlockID{BlockLocalBin, BlockStarship, BlockFzf, BlockRust, BlockGo}

const localBinBody = `case ":$PATH:" in
  *:"$HOME/.local/bin":*) ;;
  *) export PATH="$HOME/.local/bin:$PATH" ;;
esac`

const goBody = `case ":$PATH:" in
  *:/usr/local/go/bin:*) ;;
  *) export PATH="$PATH:/usr/local/go/bin" ;;
esac`

const rustBody = `[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"`

func guarded(bin, line string) string {
	return "if command -v " + bin + " >/dev/null 2>&1; then\n  " + line + "\nfi"
}

// Body is the block text for one shell, without the fence lines.
func Body(id BlockID, sh Shell) (string, error) {
	if sh != Bash && sh != Zsh {
		return "", fmt.Errorf("未知的 shell %s", sh)
	}
	switch id {
	case BlockLocalBin:
		return localBinBody, nil
	case BlockStarship:
		return guarded("starship", `eval "$(starship init `+string(sh)+`)"`), nil
	case BlockFzf:
		if sh == Zsh {
			return guarded("fzf", `source <(fzf --zsh)`), nil
		}
		return guarded("fzf", `eval "$(fzf --bash)"`), nil
	case BlockRust:
		return rustBody, nil
	case BlockGo:
		return goBody, nil
	default:
		return "", fmt.Errorf("未知的 block %s", id)
	}
}

func Begin(id BlockID) string { return "# >>> dotfiles:" + string(id) + " >>>" }
func End(id BlockID) string   { return "# <<< dotfiles:" + string(id) + " <<<" }

// Block is the fenced text, ending in a newline.
func Block(id BlockID, sh Shell) (string, error) {
	body, err := Body(id, sh)
	if err != nil {
		return "", err
	}
	return Begin(id) + "\n" + body + "\n" + End(id) + "\n", nil
}

// legacyHook maps a block to the older single-file bash hook it replaces.
var legacyHook = map[BlockID]string{
	BlockLocalBin: "local-bin",
	BlockStarship: "starship",
	BlockFzf:      "fzf",
	BlockRust:     "cargo-env",
	BlockGo:       "go-path",
}

// Order sorts and de-duplicates ids into file order. Unknown ids are an error.
func Order(ids []BlockID) ([]BlockID, error) {
	want := map[BlockID]bool{}
	for _, id := range ids {
		if _, ok := legacyHook[id]; !ok {
			return nil, fmt.Errorf("未知的 block %s", id)
		}
		want[id] = true
	}
	var out []BlockID
	for _, id := range canonical {
		if want[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// Present lists the managed blocks found in content, in file order.
func Present(content []byte) []BlockID {
	var out []BlockID
	for _, id := range canonical {
		if _, _, ok := findBlock(string(content), id); ok {
			out = append(out, id)
		}
	}
	return out
}

// findBlock returns the byte range of the first fenced block, including the
// trailing newline of the end fence when there is one.
func findBlock(s string, id BlockID) (start, end int, ok bool) {
	begin := Begin(id)
	stop := End(id)
	pos := 0
	for pos <= len(s) {
		i := strings.Index(s[pos:], begin)
		if i < 0 {
			return 0, 0, false
		}
		i += pos
		if i > 0 && s[i-1] != '\n' {
			pos = i + len(begin)
			continue
		}
		j := strings.Index(s[i:], "\n"+stop)
		if j < 0 {
			return 0, 0, false
		}
		e := i + j + 1 + len(stop)
		if e < len(s) && s[e] == '\n' {
			e++
		}
		return i, e, true
	}
	return 0, 0, false
}

func stripBlock(s string, id BlockID) (string, bool) {
	removed := false
	for {
		i, e, ok := findBlock(s, id)
		if !ok {
			return s, removed
		}
		s = s[:i] + s[e:]
		removed = true
	}
}

// stripLegacy removes the exact snippet the old bash hook writer appended.
func stripLegacy(s string, id BlockID) (string, bool) {
	name, ok := legacyHook[id]
	if !ok {
		return s, false
	}
	body, ok := hooks.Bytes(name)
	if !ok || len(body) == 0 {
		return s, false
	}
	snip := string(body)
	if !strings.HasSuffix(snip, "\n") {
		snip += "\n"
	}
	removed := false
	for {
		i := strings.Index(s, snip)
		if i < 0 || (i > 0 && s[i-1] != '\n') {
			return s, removed
		}
		s = s[:i] + s[i+len(snip):]
		removed = true
	}
}

// Ensure returns content with every id in ids (plus the managed blocks already
// present) written once, at the end, in canonical order. Applying it twice
// gives the same bytes. Legacy bash hook snippets for those ids are dropped.
func Ensure(content []byte, sh Shell, ids []BlockID) ([]byte, []BlockID, error) {
	ordered, err := Order(ids)
	if err != nil {
		return nil, nil, err
	}
	have := Present(content)
	all, _ := Order(append(append([]BlockID(nil), have...), ordered...))
	s := string(content)
	var added []BlockID
	for _, id := range all {
		var found bool
		s, found = stripBlock(s, id)
		var legacy bool
		s, legacy = stripLegacy(s, id)
		if !found && !legacy && contains(ordered, id) {
			added = append(added, id)
		}
	}
	var b strings.Builder
	b.WriteString(s)
	if b.Len() > 0 && !strings.HasSuffix(s, "\n") {
		b.WriteByte('\n')
	}
	for _, id := range all {
		blk, err := Block(id, sh)
		if err != nil {
			return nil, nil, err
		}
		b.WriteString(blk)
	}
	return []byte(b.String()), added, nil
}

// Remove drops the managed blocks for ids and any legacy bash snippets for them.
func Remove(content []byte, ids []BlockID) ([]byte, []BlockID, error) {
	ordered, err := Order(ids)
	if err != nil {
		return nil, nil, err
	}
	s := string(content)
	var removed []BlockID
	for _, id := range ordered {
		var a, b bool
		s, a = stripBlock(s, id)
		s, b = stripLegacy(s, id)
		if a || b {
			removed = append(removed, id)
		}
	}
	return []byte(s), removed, nil
}

func contains(ids []BlockID, id BlockID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// RcPath is ~/.bashrc or ~/.zshrc under home.
func RcPath(home string, sh Shell) string {
	return filepath.Join(home, "."+string(sh)+"rc")
}

// EnsureFile applies Ensure to path. A missing file is created (0644). The
// file is only rewritten when its bytes change.
func EnsureFile(path, home string, sh Shell, ids []BlockID) ([]BlockID, error) {
	old, mode, err := readRc(path, home)
	if err != nil {
		return nil, err
	}
	next, added, err := Ensure(old, sh, ids)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(old, next) {
		return added, nil
	}
	return added, writeAtomic(path, next, mode)
}

// RemoveFile applies Remove to path. A missing file is a no-op.
func RemoveFile(path, home string, ids []BlockID) ([]BlockID, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		if _, err := Order(ids); err != nil {
			return nil, err
		}
		return nil, nil
	}
	old, mode, err := readRc(path, home)
	if err != nil {
		return nil, err
	}
	next, removed, err := Remove(old, ids)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(old, next) {
		return removed, nil
	}
	return removed, writeAtomic(path, next, mode)
}

func readRc(path, home string) ([]byte, os.FileMode, error) {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "\n\x00") {
		return nil, 0, fmt.Errorf("路徑必須是絕對路徑且不得含換行或 NUL")
	}
	if home == "" {
		return nil, 0, fmt.Errorf("沒有 home")
	}
	if err := safepath.RejectEscapingSymlink(path, home); err != nil {
		return nil, 0, err
	}
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return nil, 0o644, nil
	case err != nil:
		return nil, 0, err
	case info.Mode()&os.ModeSymlink != 0:
		return nil, 0, fmt.Errorf("拒絕 symlink：%s", path)
	case !info.Mode().IsRegular():
		return nil, 0, fmt.Errorf("不是一般檔案：%s", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return body, info.Mode().Perm(), nil
}

func writeAtomic(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".rc-*")
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
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
