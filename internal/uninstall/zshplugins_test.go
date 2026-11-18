package uninstall_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

func TestRunRemovesZshPlugins(t *testing.T) {
	home, repo := fullHome(t)
	for _, p := range install.ZshPlugins {
		put(t, filepath.Join(home, ".zsh", p.Name, "x.zsh"), "x")
	}
	put(t, filepath.Join(home, ".zsh", "cache", "zcompcache"), "c")
	steps, err := uninstall.BuildSteps([]catalog.ID{"zsh"}, "dnf", 1000, uninstall.Probe{Home: home, Exists: func(p string) bool { return p == "/usr/bin/zsh" }})
	if err != nil {
		t.Fatal(err)
	}
	r := &uninstall.Runner{Exec: &fakeExec{}, Home: home, Repo: repo}
	results, code := r.Run(context.Background(), steps)
	if code != 0 {
		t.Fatalf("code %d %+v", code, results)
	}
	if _, err := os.Stat(filepath.Join(home, ".zsh")); !os.IsNotExist(err) {
		t.Fatalf("~/.zsh left behind: %v", err)
	}
	// A user's own file in ~/.zsh keeps the directory.
	put(t, filepath.Join(home, ".zsh", "mine.zsh"), "m")
	put(t, filepath.Join(home, ".zsh", "fzf-tab", "x"), "x")
	if _, code := r.Run(context.Background(), steps); code != 0 {
		t.Fatalf("rerun code %d", code)
	}
	if _, err := os.Stat(filepath.Join(home, ".zsh", "mine.zsh")); err != nil {
		t.Fatalf("user file removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zsh", "fzf-tab")); !os.IsNotExist(err) {
		t.Fatalf("plugin kept: %v", err)
	}
}
