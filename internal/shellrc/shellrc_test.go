package shellrc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/shellrc"
	"github.com/shuhi/dotfiles/shells/bash/hooks"
)

func TestBodyPerShell(t *testing.T) {
	cases := []struct {
		id   shellrc.BlockID
		sh   shellrc.Shell
		want string
	}{
		{shellrc.BlockStarship, shellrc.Bash, `eval "$(starship init bash)"`},
		{shellrc.BlockStarship, shellrc.Zsh, `eval "$(starship init zsh)"`},
		{shellrc.BlockFzf, shellrc.Bash, `eval "$(fzf --bash)"`},
		{shellrc.BlockFzf, shellrc.Zsh, `source <(fzf --zsh)`},
		{shellrc.BlockLocalBin, shellrc.Zsh, `export PATH="$HOME/.local/bin:$PATH"`},
		{shellrc.BlockRust, shellrc.Bash, `. "$HOME/.cargo/env"`},
		{shellrc.BlockGo, shellrc.Zsh, `/usr/local/go/bin`},
	}
	for _, c := range cases {
		body, err := shellrc.Body(c.id, c.sh)
		if err != nil || !strings.Contains(body, c.want) {
			t.Errorf("%s/%s = %q %v", c.id, c.sh, body, err)
		}
	}
	if _, err := shellrc.Body("nope", shellrc.Bash); err == nil {
		t.Error("unknown block accepted")
	}
	if _, err := shellrc.Body(shellrc.BlockFzf, "fish"); err == nil {
		t.Error("unknown shell accepted")
	}
}

func TestEnsureIdempotentAndOrdered(t *testing.T) {
	user := "# my stuff\nalias ll='ls -l'\n"
	for _, sh := range shellrc.Shells {
		out, added, err := shellrc.Ensure([]byte(user), sh, []shellrc.BlockID{shellrc.BlockStarship})
		if err != nil {
			t.Fatal(err)
		}
		if len(added) != 1 || added[0] != shellrc.BlockStarship {
			t.Fatalf("added %v", added)
		}
		if !strings.HasPrefix(string(out), user) {
			t.Fatalf("user lines moved:\n%s", out)
		}
		// a later run adds local-bin: it must land before starship.
		out2, added2, err := shellrc.Ensure(out, sh, []shellrc.BlockID{shellrc.BlockLocalBin, shellrc.BlockStarship})
		if err != nil {
			t.Fatal(err)
		}
		if len(added2) != 1 || added2[0] != shellrc.BlockLocalBin {
			t.Fatalf("added2 %v", added2)
		}
		s := string(out2)
		if strings.Index(s, shellrc.Begin(shellrc.BlockLocalBin)) > strings.Index(s, shellrc.Begin(shellrc.BlockStarship)) {
			t.Fatalf("order:\n%s", s)
		}
		if strings.Count(s, shellrc.Begin(shellrc.BlockStarship)) != 1 {
			t.Fatalf("duplicate block:\n%s", s)
		}
		out3, added3, _ := shellrc.Ensure(out2, sh, []shellrc.BlockID{shellrc.BlockStarship, shellrc.BlockLocalBin})
		if string(out3) != s || len(added3) != 0 {
			t.Fatalf("not idempotent:\n%s", out3)
		}
	}
}

func TestEnsureNoTrailingNewline(t *testing.T) {
	out, _, err := shellrc.Ensure([]byte("export A=1"), shellrc.Bash, []shellrc.BlockID{shellrc.BlockGo})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "export A=1\n# >>> dotfiles:go >>>\n") {
		t.Fatalf("%q", out)
	}
}

func TestEnsureMigratesLegacyHook(t *testing.T) {
	legacy, _ := hooks.Bytes("starship")
	in := "x=1\n" + string(legacy) + "y=2\n"
	out, added, err := shellrc.Ensure([]byte(in), shellrc.Bash, []shellrc.BlockID{shellrc.BlockStarship})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Fatalf("legacy hook counted as new: %v", added)
	}
	s := string(out)
	if strings.Contains(s, "# dotfiles:starship\n") || strings.Count(s, "starship init bash") != 1 {
		t.Fatalf("legacy not replaced:\n%s", s)
	}
	if !strings.HasPrefix(s, "x=1\ny=2\n") {
		t.Fatalf("user lines:\n%s", s)
	}
}

func TestRemoveExactlyTheBlock(t *testing.T) {
	user := "# top\nexport EDITOR=vim\n"
	cases := []struct {
		name   string
		ensure []shellrc.BlockID
		remove []shellrc.BlockID
		keep   []shellrc.BlockID
	}{
		{"one of two", []shellrc.BlockID{shellrc.BlockLocalBin, shellrc.BlockStarship}, []shellrc.BlockID{shellrc.BlockStarship}, []shellrc.BlockID{shellrc.BlockLocalBin}},
		{"all", []shellrc.BlockID{shellrc.BlockFzf, shellrc.BlockRust}, []shellrc.BlockID{shellrc.BlockFzf, shellrc.BlockRust}, nil},
		{"absent", []shellrc.BlockID{shellrc.BlockGo}, []shellrc.BlockID{shellrc.BlockFzf}, []shellrc.BlockID{shellrc.BlockGo}},
	}
	for _, c := range cases {
		for _, sh := range shellrc.Shells {
			in, _, err := shellrc.Ensure([]byte(user), sh, c.ensure)
			if err != nil {
				t.Fatal(err)
			}
			out, _, err := shellrc.Remove(in, c.remove)
			if err != nil {
				t.Fatal(err)
			}
			got := shellrc.Present(out)
			if strings.Join(ids(got), ",") != strings.Join(ids(c.keep), ",") {
				t.Errorf("%s/%s present %v", c.name, sh, got)
			}
			if !strings.HasPrefix(string(out), user) {
				t.Errorf("%s/%s user lines lost:\n%s", c.name, sh, out)
			}
			if len(c.keep) == 0 && string(out) != user {
				t.Errorf("%s/%s leftovers %q", c.name, sh, out)
			}
		}
	}
}

func TestRemoveLegacyHook(t *testing.T) {
	legacy, _ := hooks.Bytes("cargo-env")
	in := "a\n" + string(legacy) + "b\n"
	out, removed, err := shellrc.Remove([]byte(in), []shellrc.BlockID{shellrc.BlockRust})
	if err != nil || string(out) != "a\nb\n" || len(removed) != 1 {
		t.Fatalf("%q %v %v", out, removed, err)
	}
}

func TestFilesBothShells(t *testing.T) {
	home := t.TempDir()
	bashrc := shellrc.RcPath(home, shellrc.Bash)
	zshrc := shellrc.RcPath(home, shellrc.Zsh)
	if err := os.WriteFile(bashrc, []byte("# keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sh := range shellrc.Shells {
		if _, err := shellrc.EnsureFile(shellrc.RcPath(home, sh), home, sh, []shellrc.BlockID{shellrc.BlockStarship}); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(bashrc)
	z, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(b), "starship init bash") || !strings.HasPrefix(string(b), "# keep\n") {
		t.Fatalf("bashrc %q", b)
	}
	if !strings.Contains(string(z), "starship init zsh") {
		t.Fatalf("zshrc %q", z)
	}
	if info, _ := os.Stat(bashrc); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed %v", info.Mode())
	}
	if info, _ := os.Stat(zshrc); info.Mode().Perm() != 0o644 {
		t.Fatalf("new zshrc mode %v", info.Mode())
	}
	for _, sh := range shellrc.Shells {
		if _, err := shellrc.RemoveFile(shellrc.RcPath(home, sh), home, []shellrc.BlockID{shellrc.BlockStarship}); err != nil {
			t.Fatal(err)
		}
	}
	b, _ = os.ReadFile(bashrc)
	z, _ = os.ReadFile(zshrc)
	if string(b) != "# keep\n" || string(z) != "" {
		t.Fatalf("after remove %q %q", b, z)
	}
	// missing file: no-op, no file created
	other := t.TempDir()
	if _, err := shellrc.RemoveFile(filepath.Join(other, ".zshrc"), other, []shellrc.BlockID{shellrc.BlockFzf}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, ".zshrc")); !os.IsNotExist(err) {
		t.Fatal("RemoveFile created a file")
	}
}

func TestFileRejectsSymlinkAndOutside(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "real")
	os.WriteFile(target, nil, 0o644)
	link := filepath.Join(home, ".bashrc")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if _, err := shellrc.EnsureFile(link, home, shellrc.Bash, []shellrc.BlockID{shellrc.BlockFzf}); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := shellrc.EnsureFile(filepath.Join(t.TempDir(), ".bashrc"), home, shellrc.Bash, []shellrc.BlockID{shellrc.BlockFzf}); err == nil {
		t.Fatal("outside home accepted")
	}
}

func ids(in []shellrc.BlockID) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = string(id)
	}
	return out
}
