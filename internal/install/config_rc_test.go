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
		if !strings.Contains(body, shellrc.Begin(shellrc.BlockBase)) || !strings.Contains(body, "starship init "+name) || !strings.Contains(body, ".local/bin") {
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

// The ~/.zshrc template replaces an existing ~/.zshrc (backed up first); the
// managed blocks the old file had (fzf…) are carried over, and blocks the
// template itself provides (its "# dotfiles:provides" line) are not added.
func TestRunZshTemplateReplacesWithBackup(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"zsh"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	const tmpl = "# zsh template\n# dotfiles:provides base local-bin starship rust go\n"
	fzf := shellrc.Begin(shellrc.BlockFzf) + "\nfzf\n" + shellrc.End(shellrc.BlockFzf) + "\n"
	now := time.Date(2026, 11, 18, 1, 2, 3, 0, time.UTC)
	for _, existing := range []string{"", "# mine\n" + fzf} {
		home := t.TempDir()
		repo := miniRepo(t)
		put(t, filepath.Join(repo, "shells", "zsh", "zshrc"), tmpl)
		if existing != "" {
			put(t, filepath.Join(home, ".zshrc"), existing)
		}
		steps, _ := install.BuildSteps(plans, 0, home)
		var cloned []string
		r := &install.Runner{
			Exec: &fakeExec{fn: func(argv []string) (string, int, error) {
				if len(argv) > 1 && argv[0] == "git" {
					cloned = append(cloned, argv[len(argv)-1])
				}
				return "", 0, nil
			}},
			Home:   home,
			Repo:   repo,
			Exists: func(string) bool { return false },
			Deploy: configcopy.Deploy,
			Rc:     shellrc.EnsureFile,
			Now:    func() time.Time { return now },
		}
		if _, code := r.RunSteps(context.Background(), steps); code != 0 {
			t.Fatalf("code %d", code)
		}
		got, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
		s := string(got)
		if !strings.HasPrefix(s, tmpl) || strings.Contains(s, shellrc.Begin(shellrc.BlockBase)) || strings.Contains(s, "# mine") {
			t.Fatalf("existing %q -> %q", existing, s)
		}
		baks, _ := filepath.Glob(filepath.Join(home, ".zshrc.bak.*"))
		if existing == "" {
			if len(baks) != 0 {
				t.Fatalf("backup without an old file: %v", baks)
			}
		} else {
			if strings.Count(s, shellrc.Begin(shellrc.BlockFzf)) != 1 {
				t.Fatalf("fzf block lost: %q", s)
			}
			if len(baks) != 1 {
				t.Fatalf("backups %v", baks)
			}
			old, _ := os.ReadFile(baks[0])
			if string(old) != existing {
				t.Fatalf("backup %q", old)
			}
		}
		if len(cloned) != 3 {
			t.Fatalf("plugins cloned %v", cloned)
		}
		if info, err := os.Stat(filepath.Join(home, ".zsh", "cache")); err != nil || !info.IsDir() {
			t.Fatalf("cache dir %v", err)
		}
		// Rerun: same bytes, no second backup, no re-clone of existing plugins.
		for _, p := range install.ZshPlugins {
			put(t, filepath.Join(home, ".zsh", p.Name, "README"), "x")
		}
		cloned = nil
		if _, code := r.RunSteps(context.Background(), steps); code != 0 {
			t.Fatalf("rerun code %d", code)
		}
		again, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
		baks2, _ := filepath.Glob(filepath.Join(home, ".zshrc.bak.*"))
		if string(again) != s || len(baks2) != len(baks) || len(cloned) != 0 {
			t.Fatalf("rerun changed: %q baks=%v cloned=%v", again, baks2, cloned)
		}
	}
}

// bashrc and a missing zshrc both get the base block (PATH, cargo, eza,
// starship init <shell>) on any install run, idempotently.
func TestRunBaseBlockBothShells(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"bat"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	fedora := "# .bashrc\n\nif [ -f /etc/bashrc ]; then\n    . /etc/bashrc\nfi\n"
	put(t, filepath.Join(home, ".bashrc"), fedora)
	steps, _ := install.BuildSteps(plans, 0, home)
	r := &install.Runner{
		Exec:   &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }},
		Home:   home,
		Exists: func(string) bool { return false },
		Rc:     shellrc.EnsureFile,
	}
	for run := 0; run < 2; run++ {
		if _, code := r.RunSteps(context.Background(), steps); code != 0 {
			t.Fatalf("code %d", code)
		}
	}
	bash, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	zsh, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("zshrc not created: %v", err)
	}
	if !strings.HasPrefix(string(bash), fedora) {
		t.Fatalf("fedora lines lost: %q", bash)
	}
	for name, body := range map[string]string{"bash": string(bash), "zsh": string(zsh)} {
		if strings.Count(body, shellrc.Begin(shellrc.BlockBase)) != 1 {
			t.Fatalf("%s: base block count: %q", name, body)
		}
		for _, want := range []string{`eval "$(starship init ` + name + `)"`, "$HOME/.local/bin", "/usr/local/go/bin", "$GOPATH/bin", ".cargo/env", "alias ls='eza"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s rc lacks %q:\n%s", name, want, body)
			}
		}
	}
}

// --config-only: seeded "ok" results let the config and zsh-plugins steps run
// with no package step; the only command run is git clone, and a rerun
// changes nothing (no new backup, no clone).
func TestRunConfigOnlyDeploysAndClones(t *testing.T) {
	steps, err := install.BuildConfigSteps([]catalog.ID{"zsh", "starship", "zellij"}, nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	repo := miniRepo(t)
	put(t, filepath.Join(home, ".zshrc"), "# mine\n")
	var argvs [][]string
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		argvs = append(argvs, argv)
		if argv[0] == "git" {
			return "", 0, os.MkdirAll(argv[len(argv)-1], 0o755)
		}
		return "", 0, nil
	}}
	now := time.Date(2026, 11, 18, 1, 2, 3, 0, time.UTC)
	seed := []install.ItemResult{{ID: "zsh", Status: "ok"}, {ID: "starship", Status: "ok"}, {ID: "zellij", Status: "ok"}}
	r := &install.Runner{Exec: ex, Home: home, Repo: repo, Deploy: configcopy.Deploy, Seed: seed, Now: func() time.Time { return now }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 3 {
		t.Fatalf("code %d %+v", code, results)
	}
	for _, rel := range []string{".zshrc", ".config/starship.toml", ".config/zellij/config.kdl", ".zsh/fzf-tab", ".zsh/zsh-autosuggestions", ".zsh/zsh-syntax-highlighting"} {
		if _, err := os.Stat(filepath.Join(home, rel)); err != nil {
			t.Error(err)
		}
	}
	if len(argvs) != len(install.ZshPlugins) {
		t.Fatalf("argvs %v", argvs)
	}
	for _, a := range argvs {
		if a[0] != "git" || a[1] != "clone" {
			t.Fatalf("unexpected command %v", a)
		}
	}
	baks, _ := filepath.Glob(filepath.Join(home, ".zshrc.bak.*"))
	if len(baks) != 1 {
		t.Fatalf("backups %v", baks)
	}
	argvs = nil
	r.Now = func() time.Time { return now.Add(time.Hour) }
	if _, code := r.RunSteps(context.Background(), steps); code != 0 || len(argvs) != 0 {
		t.Fatalf("rerun code %d argvs %v", code, argvs)
	}
	baks, _ = filepath.Glob(filepath.Join(home, "*.bak.*"))
	more, _ := filepath.Glob(filepath.Join(home, ".config", "*", "*.bak.*"))
	if len(baks)+len(more) != 1 {
		t.Fatalf("rerun made backups %v %v", baks, more)
	}
}

// --config-only with git missing: when installing git fails, the configs are
// still deployed, git is reported failed (exit 1, as for any package), and
// zsh-plugins reports the clones it could not make.
func TestRunConfigOnlyGitInstallFails(t *testing.T) {
	git, err := install.ResolveAll([]catalog.ID{"git"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildConfigSteps([]catalog.ID{"zsh", "zellij"}, git, 1000)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		if argv[0] == "sudo" {
			return "sudo: a password is required", 1, nil
		}
		return "git: not found", 127, nil
	}}
	r := &install.Runner{
		Exec: ex, Home: home, Repo: miniRepo(t), Manager: "dnf", Deploy: configcopy.Deploy,
		Exists: func(string) bool { return false },
		Seed:   []install.ItemResult{{ID: "zsh", Status: "ok"}, {ID: "zellij", Status: "ok"}},
	}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 {
		t.Fatalf("code %d %+v", code, results)
	}
	for _, rel := range []string{".zshrc", ".config/zellij/config.kdl"} {
		if _, err := os.Stat(filepath.Join(home, rel)); err != nil {
			t.Error(err)
		}
	}
	byID := map[catalog.ID]install.ItemResult{}
	for _, res := range results {
		byID[res.ID] = res
	}
	if byID["git"].Status != "failed" || byID["git"].ErrText == "" {
		t.Fatalf("git %+v", byID["git"])
	}
	if !strings.Contains(byID["zsh"].Note, "zsh 外掛下載失敗") {
		t.Fatalf("zsh %+v", byID["zsh"])
	}
	if len(ex.calls) == 0 || strings.Join(ex.calls[0], " ") != "sudo dnf install -y git" {
		t.Fatalf("calls %v", ex.calls)
	}
}
