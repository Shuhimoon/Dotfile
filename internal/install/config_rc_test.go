package install_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func miniRepo(t *testing.T) string {
	repo := t.TempDir()
	put(t, filepath.Join(repo, "starship", "starship.toml"), "theme\n")
	put(t, filepath.Join(repo, "zellij", "config.kdl"), "cfg\n")
	put(t, filepath.Join(repo, "zellij", "layouts", "default.kdl"), "layout\n")
	put(t, filepath.Join(repo, "zellij", "plugins", "zjstatus.wasm"), "wasm")
	put(t, filepath.Join(repo, "shells", "zsh", "zshrc"), "# zsh template\n")
	return repo
}

func TestRunDeploysConfigAndBothRcFiles(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"starship", "zellij"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	steps, err := install.BuildSteps(plans, 0, home)
	if err != nil {
		t.Fatal(err)
	}
	repo := miniRepo(t)
	// an existing, different starship.toml must be backed up, not clobbered
	put(t, filepath.Join(home, ".config", "starship.toml"), "mine\n")
	put(t, filepath.Join(home, ".bashrc"), "# user bashrc\n")
	now := time.Date(2026, 11, 17, 1, 2, 3, 0, time.UTC)
	r := &install.Runner{
		Home:    home,
		Repo:    repo,
		Exists:  func(string) bool { return false },
		Release: func(context.Context, string, string, release.AssetQuery) error { return nil },
		Deploy:  configcopy.Deploy,
		Rc:      shellrc.EnsureFile,
		Now:     func() time.Time { return now },
	}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 {
		t.Fatalf("code %d %+v", code, results)
	}
	want := map[string]string{
		".config/starship.toml":                      "theme\n",
		".config/starship.toml.bak.20261117T010203Z": "mine\n",
		".config/zellij/config.kdl":                  "cfg\n",
		".config/zellij/layouts/default.kdl":         "layout\n",
		".config/zellij/plugins/zjstatus.wasm":       "wasm",
	}
	for rel, body := range want {
		got, err := os.ReadFile(filepath.Join(home, rel))
		if err != nil || string(got) != body {
			t.Errorf("%s = %q %v", rel, got, err)
		}
	}
	bash, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	zsh, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
	for name, body := range map[string]string{"bash": string(bash), "zsh": string(zsh)} {
		if !strings.Contains(body, shellrc.Begin(shellrc.BlockLocalBin)) || !strings.Contains(body, "starship init "+name) {
			t.Errorf("%src:\n%s", name, body)
		}
	}
	if !strings.HasPrefix(string(bash), "# user bashrc\n") {
		t.Errorf("bashrc user lines lost")
	}
	// second run: same bytes everywhere (idempotent), no new backup
	results, code = r.RunSteps(context.Background(), steps)
	if code != 0 {
		t.Fatalf("rerun code %d %+v", code, results)
	}
	bash2, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if string(bash2) != string(bash) {
		t.Errorf("bashrc changed on rerun")
	}
	baks, _ := filepath.Glob(filepath.Join(home, ".config", "starship.toml.bak.*"))
	if len(baks) != 1 {
		t.Errorf("backups %v", baks)
	}
}

func TestRunZshTemplateOnlyIfMissing(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"zsh"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	for _, existing := range []string{"", "# mine\n"} {
		home := t.TempDir()
		if existing != "" {
			put(t, filepath.Join(home, ".zshrc"), existing)
		}
		steps, _ := install.BuildSteps(plans, 0, home)
		r := &install.Runner{
			Exec:   &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }},
			Home:   home,
			Repo:   miniRepo(t),
			Exists: func(string) bool { return false },
			Deploy: configcopy.Deploy,
			Rc:     shellrc.EnsureFile,
		}
		if _, code := r.RunSteps(context.Background(), steps); code != 0 {
			t.Fatalf("code %d", code)
		}
		got, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
		want := existing
		if want == "" {
			want = "# zsh template\n"
		}
		if string(got) != want {
			t.Errorf("existing %q -> %q", existing, got)
		}
	}
}
