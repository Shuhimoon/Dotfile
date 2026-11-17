package linkbash

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/safepath"
	"github.com/shuhi/dotfiles/shells/bash/hooks"
)

type HookID string

const (
	HookLocalBin HookID = "local-bin"
	HookStarship HookID = "starship"
	HookFzf      HookID = "fzf"
	HookCargoEnv HookID = "cargo-env"
	HookGoPath   HookID = "go-path"
)

type Report struct {
	Added   []HookID
	Skipped []HookID
}

var canonical = []HookID{HookLocalBin, HookStarship, HookFzf, HookCargoEnv, HookGoPath}

// Payloads are the single lines that count as "already present".
// The case-statement arms inside local-bin and go-path are not payloads.
var payloads = map[HookID]string{
	HookStarship: `eval "$(starship init bash)"`,
	HookFzf:      `eval "$(fzf --bash)"`,
	HookLocalBin: `export PATH="$HOME/.local/bin:$PATH"`,
	HookCargoEnv: `. "$HOME/.cargo/env"`,
	HookGoPath:   `export PATH="$PATH:/usr/local/go/bin"`,
}

func Order(in []HookID) ([]HookID, error) {
	seen := map[HookID]bool{}
	for _, id := range in {
		if _, ok := payloads[id]; !ok {
			return nil, fmt.Errorf("未知的 hook %s", id)
		}
	}
	var out []HookID
	for _, id := range canonical {
		for _, got := range in {
			if got == id && !seen[id] {
				out = append(out, id)
				seen[id] = true
			}
		}
	}
	return out, nil
}

// Apply appends hook snippets to path. root is os.UserHomeDir, not a parameter.
// There is no require-cmd filter here.
func Apply(path string, ids []HookID) (Report, error) {
	ordered, err := Order(ids)
	if err != nil {
		return Report{}, err
	}
	if !filepath.IsAbs(path) || strings.ContainsRune(path, '\n') || strings.ContainsRune(path, 0) {
		return Report{}, fmt.Errorf("路徑必須是絕對路徑且不得含換行或 NUL")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Report{}, err
	}
	if err := safepath.RejectEscapingSymlink(path, home); err != nil {
		return Report{}, err
	}

	var existing []byte
	var mode os.FileMode = 0o644
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return Report{}, fmt.Errorf("拒絕 symlink：%s", path)
		}
		mode = info.Mode().Perm()
		existing, err = os.ReadFile(path)
		if err != nil {
			return Report{}, err
		}
	case os.IsNotExist(err):
	default:
		return Report{}, err
	}

	var report Report
	var buf bytes.Buffer
	buf.Write(existing)
	for _, id := range ordered {
		body, ok := hooks.Bytes(string(id))
		if !ok {
			return Report{}, fmt.Errorf("未知的 hook %s", id)
		}
		marker := firstLine(body)
		if hasLine(existing, marker) || hasLine(existing, payloads[id]) {
			report.Skipped = append(report.Skipped, id)
			continue
		}
		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteByte('\n')
		}
		buf.Write(body)
		if len(body) == 0 || body[len(body)-1] != '\n' {
			buf.WriteByte('\n')
		}
		report.Added = append(report.Added, id)
		existing = append([]byte{}, buf.Bytes()...)
	}
	if len(report.Added) == 0 {
		return report, nil
	}
	if err := writeAtomic(path, buf.Bytes(), mode); err != nil {
		return Report{}, err
	}
	return report, nil
}

func firstLine(b []byte) string {
	line, _, _ := bytes.Cut(b, []byte("\n"))
	return strings.TrimSpace(string(line))
}

func hasLine(content []byte, want string) bool {
	if want == "" {
		return false
	}
	for _, line := range bytes.Split(content, []byte("\n")) {
		if strings.TrimSpace(string(line)) == want {
			return true
		}
	}
	return false
}

func writeAtomic(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".bashrc-*")
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
