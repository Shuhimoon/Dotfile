package main

import (
	"fmt"
	"io"
	"os"
	"os/user"

	"github.com/shuhi/dotfiles/internal/sudouser"
)

// sudoSetup handles `sudo ./tools`: HOME is pointed back at the user who ran
// sudo, so ~/.local/bin, ~/.config, ~/.bashrc, ~/.zshrc, ~/.zsh, ~/.ssh and
// ~/.cargo are theirs, and fix gives the files written as root back to them.
// A real root login (no SUDO_USER) keeps root's home; fix is then a no-op.
func sudoSetup(stderr io.Writer) (fix func(), target string, err error) {
	fix = func() {}
	t, ok, err := sudouser.Detect(os.Geteuid(), os.Getenv, user.Lookup)
	if err != nil || !ok {
		return fix, "", err
	}
	rootHome := "/root"
	if u, err := user.LookupId("0"); err == nil && u.HomeDir != "" {
		rootHome = u.HomeDir
	}
	_ = os.Setenv(sudouser.RootHomeEnv, rootHome)
	if err := t.Env(os.Setenv, os.Getenv, os.Unsetenv); err != nil {
		return fix, "", err
	}
	fixer := sudouser.NewFixer(t)
	fix = func() {
		roots := append(sudouser.Roots(t.Home), fixer.TopLevel(t.Home)...)
		if _, err := fixer.Fix(roots); err != nil {
			fmt.Fprintln(stderr, "修正檔案擁有者失敗:", err)
		}
	}
	return fix, t.Name, nil
}
