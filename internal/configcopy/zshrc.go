package configcopy

import (
	"bytes"
	"os"
	"time"

	"github.com/shuhi/dotfiles/internal/shellrc"
)

// sameRc is true when have is the template plus managed blocks only.
func sameRc(have, template []byte) bool {
	a := bytes.TrimRight(shellrc.StripManaged(have), "\n")
	b := bytes.TrimRight(template, "\n")
	return bytes.Equal(a, b)
}

func deployZshrc(dst string, body []byte, exists bool, now time.Time) (Result, error) {
	if !exists {
		if err := writeAtomic(dst, body, 0o644); err != nil {
			return Result{}, err
		}
		return Result{Action: "created"}, nil
	}
	old, err := os.ReadFile(dst)
	if err != nil {
		return Result{}, err
	}
	if sameRc(old, body) {
		return Result{Action: "unchanged"}, nil
	}
	next, _, err := shellrc.Ensure(body, shellrc.Zsh, shellrc.Present(old))
	if err != nil {
		return Result{}, err
	}
	if err := backup(dst, now); err != nil {
		return Result{}, err
	}
	if err := writeAtomic(dst, next, 0o644); err != nil {
		return Result{}, err
	}
	return Result{Action: "backed_up"}, nil
}
