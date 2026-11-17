package uninstall_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

func TestWindowsRemovable(t *testing.T) {
	for _, id := range []catalog.ID{"wezterm", "python", "git"} {
		if !uninstall.RemovableOn("windows", id) || !uninstall.SupportedOn("windows", id, "scoop") {
			t.Fatalf("%s not removable on windows", id)
		}
	}
	if uninstall.RemovableOn("linux", "git") {
		t.Fatal("git is locked on linux")
	}
	if uninstall.RemovableOn("windows", "jq") {
		t.Fatal("jq is not a windows item")
	}
	if _, warn := uninstall.WarningOn("windows", "python"); warn {
		t.Fatal("dnf python warning shown on windows")
	}
	if _, warn := uninstall.WarningOn("linux", "python"); !warn {
		t.Fatal("python warning lost on linux")
	}
}

func TestBuildWindowsUninstallSteps(t *testing.T) {
	steps, err := uninstall.BuildWindowsSteps([]catalog.ID{"git", "wezterm"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, st := range steps {
		got = append(got, st.Title+"|"+strings.Join(st.Argv, " "))
	}
	want := []string{"wezterm|scoop uninstall wezterm", "git|scoop uninstall git", "ssh-config git|"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	if _, err := uninstall.BuildWindowsSteps([]catalog.ID{"jq"}); err == nil {
		t.Fatal("jq accepted")
	}
}

func TestRunWindowsUninstallGit(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".ssh", "config")
	put(t, cfg, "# >>> dotfiles:github >>>\nHost github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/gh\n# <<< dotfiles:github <<<\n")
	put(t, filepath.Join(home, ".ssh", "gh"), "key")
	steps, _ := uninstall.BuildWindowsSteps([]catalog.ID{"git"})
	ex := &fakeExec{}
	results, code := (&uninstall.Runner{Exec: ex, Home: home}).Run(context.Background(), steps)
	if code != 0 || len(ex.calls) != 1 || strings.Join(ex.calls[0], " ") != "scoop uninstall git" {
		t.Fatalf("code %d calls %v results %+v", code, ex.calls, results)
	}
	if b, _ := os.ReadFile(cfg); strings.Contains(string(b), "github.com") {
		t.Fatalf("block kept: %q", b)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "gh")); err != nil {
		t.Fatal("key removed")
	}
}
