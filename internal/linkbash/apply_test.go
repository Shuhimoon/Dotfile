package linkbash_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/linkbash"
	"github.com/shuhi/dotfiles/shells/bash/hooks"
)

func TestStarshipSnippet(t *testing.T) {
	want := "# dotfiles:starship\neval \"$(starship init bash)\"\n"
	if string(hooks.Starship) != want {
		t.Fatalf("%q", hooks.Starship)
	}
}

func TestFzfSnippet(t *testing.T) {
	want := "# dotfiles:fzf\neval \"$(fzf --bash)\"\n"
	if string(hooks.Fzf) != want {
		t.Fatalf("%q", hooks.Fzf)
	}
}

func TestLocalBinSnippet(t *testing.T) {
	want := "# dotfiles:local-bin\ncase \":$PATH:\" in\n  *:\"$HOME/.local/bin\":*) ;;\n  *) export PATH=\"$HOME/.local/bin:$PATH\" ;;\nesac\n"
	if string(hooks.LocalBin) != want {
		t.Fatalf("%q", hooks.LocalBin)
	}
}

func TestCargoEnvSnippet(t *testing.T) {
	want := "# dotfiles:cargo-env\n. \"$HOME/.cargo/env\"\n"
	if string(hooks.CargoEnv) != want {
		t.Fatalf("%q", hooks.CargoEnv)
	}
}

func TestGoPathSnippet(t *testing.T) {
	want := "# dotfiles:go-path\ncase \":$PATH:\" in\n  *:/usr/local/go/bin:*) ;;\n  *) export PATH=\"$PATH:/usr/local/go/bin\" ;;\nesac\n"
	if string(hooks.GoPath) != want {
		t.Fatalf("%q", hooks.GoPath)
	}
}

func TestApplyCreatesFile(t *testing.T) {
	path := bashrc(t)
	rep, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || rep.Added[0] != linkbash.HookStarship {
		t.Fatalf("added %v", rep.Added)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(hooks.Starship) {
		t.Fatalf("%q", got)
	}
}

func TestApplyIdempotent(t *testing.T) {
	path := bashrc(t)
	if err := os.WriteFile(path, []byte("# user\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship, linkbash.HookCargoEnv}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode not preserved: %v", info)
	}
	first, _ := os.ReadFile(path)
	rep, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship, linkbash.HookCargoEnv})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 0 || len(rep.Skipped) != 2 {
		t.Fatalf("second %+v", rep)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatalf("changed\n%s", second)
	}
}

func TestApplyDoesNotDuplicateExistingUpstreamLine(t *testing.T) {
	cases := []struct {
		id   linkbash.HookID
		line string
	}{
		{linkbash.HookStarship, `eval "$(starship init bash)"`},
		{linkbash.HookFzf, `eval "$(fzf --bash)"`},
		{linkbash.HookLocalBin, `export PATH="$HOME/.local/bin:$PATH"`},
		{linkbash.HookCargoEnv, `. "$HOME/.cargo/env"`},
		{linkbash.HookGoPath, `export PATH="$PATH:/usr/local/go/bin"`},
	}
	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			path := bashrc(t)
			if err := os.WriteFile(path, []byte(tc.line+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			rep, err := linkbash.Apply(path, []linkbash.HookID{tc.id})
			if err != nil {
				t.Fatal(err)
			}
			if len(rep.Added) != 0 || len(rep.Skipped) != 1 {
				t.Fatalf("%+v", rep)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tc.line+"\n" {
				t.Fatalf("appended %q", got)
			}
		})
	}
}

func TestApplyOrdersHooks(t *testing.T) {
	path := bashrc(t)
	in := []linkbash.HookID{linkbash.HookGoPath, linkbash.HookFzf, linkbash.HookLocalBin, linkbash.HookFzf}
	if _, err := linkbash.Apply(path, in); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	local := strings.Index(string(got), "# dotfiles:local-bin")
	fzf := strings.Index(string(got), "# dotfiles:fzf")
	goPath := strings.Index(string(got), "# dotfiles:go-path")
	if local < 0 || fzf < 0 || goPath < 0 || !(local < fzf && fzf < goPath) {
		t.Fatalf("order\n%s", got)
	}
	if strings.Count(string(got), "# dotfiles:fzf") != 1 {
		t.Fatalf("duplicate fzf\n%s", got)
	}
}

func TestRejectsParentSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	_, err := linkbash.Apply(filepath.Join(dir, "link", ".bashrc"), []linkbash.HookID{linkbash.HookStarship})
	if err == nil || !strings.Contains(err.Error(), "拒絕 symlink：") {
		t.Fatalf("err = %v", err)
	}
}

func TestCommentedLineDoesNotCount(t *testing.T) {
	path := bashrc(t)
	if err := os.WriteFile(path, []byte("# eval \"$(starship init bash)\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 {
		t.Fatalf("%+v", rep)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "eval \"$(starship init bash)\"") {
		t.Fatalf("%s", got)
	}
}

func TestMarkerWithoutPayloadDoesNotReadd(t *testing.T) {
	path := bashrc(t)
	body := "# dotfiles:starship\n# payload removed\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 0 || len(rep.Skipped) != 1 {
		t.Fatalf("%+v", rep)
	}
	got, _ := os.ReadFile(path)
	if string(got) != body {
		t.Fatalf("rewritten %q", got)
	}
}

func TestRejectsSymlinkBashrc(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	real := filepath.Join(dir, "real")
	path := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(real, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, path); err != nil {
		t.Fatal(err)
	}
	_, err := linkbash.Apply(path, []linkbash.HookID{linkbash.HookStarship})
	if err == nil || !strings.Contains(err.Error(), "拒絕 symlink：") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectsPathWithNewline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	_, err := linkbash.Apply(filepath.Join(dir, ".bashrc")+"\n", []linkbash.HookID{linkbash.HookStarship})
	if err == nil {
		t.Fatal("expected newline path to be rejected")
	}
	_, err = linkbash.Apply("relative/.bashrc", []linkbash.HookID{linkbash.HookStarship})
	if err == nil {
		t.Fatal("expected relative path to be rejected")
	}
	if _, err := linkbash.Apply(filepath.Join(dir, ".bashrc"), []linkbash.HookID{"nope"}); err == nil {
		t.Fatal("expected unknown hook to fail")
	}
}

func bashrc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return filepath.Join(dir, ".bashrc")
}
