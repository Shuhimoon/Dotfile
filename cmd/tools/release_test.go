package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	old := [3]string{version, commit, date}
	t.Cleanup(func() { version, commit, date = old[0], old[1], old[2] })
	version, commit, date = "0.1.0", "abc1234", "2026-11-17T08:00:00Z"
	for _, arg := range []string{"--version", "version"} {
		in, stdout, stderr := tempIO(t)
		if code := run([]string{arg}, in, stdout, stderr); code != 0 {
			t.Fatalf("%s code %d", arg, code)
		}
		if got := readFromStart(t, stdout); got != "tools 0.1.0 (commit abc1234, built 2026-11-17T08:00:00Z)\n" {
			t.Fatalf("%s %q", arg, got)
		}
	}
	if !strings.Contains(usage, "tools --version") {
		t.Fatal("usage")
	}
}

func TestFindRepoSkipsOtherModules(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "dotfiles")
	other := filepath.Join(repo, "sub", "othermod")
	if err := os.MkdirAll(filepath.Join(other, "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module github.com/shuhi/dotfiles\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "go.mod"), []byte("module example.com/other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findRepoFrom(filepath.Join(other, "deep"))
	if err != nil || got != repo {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := findRepoFrom(root); err == nil {
		t.Fatal("found a repo outside the checkout")
	}
}

// TestReleasedBinaryUsesEmbeddedConfigs: outside a checkout the configs come
// from the binary, extracted under ~/.local/share/dotfiles-tools/configs/<version>.
func TestReleasedBinaryUsesEmbeddedConfigs(t *testing.T) {
	home := t.TempDir()
	dir, err := extractConfigs(home, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "dotfiles-tools", "configs", "0.1.0")
	if dir != want {
		t.Fatalf("dir %q", dir)
	}
	for _, f := range []string{"starship/starship.toml", "zellij/config.kdl", "zellij/plugins/zjstatus.wasm", "shells/zsh/zshrc"} {
		if fi, err := os.Stat(filepath.Join(dir, f)); err != nil || fi.Size() == 0 {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if dir, err := extractConfigs("", "0.1.0"); dir != "" || err != nil {
		t.Fatal("no home")
	}
	// an extract failure is returned (with its reason), not printed
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if dir, err := extractConfigs(blocked, "0.1.0"); dir != "" || err == nil || !strings.Contains(err.Error(), "設定檔解壓失敗") {
		t.Fatalf("blocked home: %q %v", dir, err)
	}
	if got := safeVersion("../../x"); strings.Contains(got, "/") || got == ".." {
		t.Fatalf("safeVersion %q", got)
	}
	if safeVersion("") != "dev" {
		t.Fatal("empty version")
	}
}
