package configcopy_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/configcopy"
)

const zshBody = "# Deployed only when ~/.zshrc does not already exist.\nexport PATH=\"$HOME/.local/bin:$PATH\"\nif command -v starship >/dev/null 2>&1; then\n  eval \"$(starship init zsh)\"\nfi\n"

func TestCopyZellij(t *testing.T) {
	root := repoRoot(t)
	got, err := os.ReadFile(filepath.Join(root, "zellij", "config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	// The repo config is the user's own; check what the deploy relies on, not every byte.
	if len(got) == 0 || !strings.Contains(string(got), "file:~/.config/zellij/plugins/zjstatus.wasm") {
		t.Fatalf("zellij config does not load the shipped zjstatus plugin")
	}
	if _, err := os.Stat(filepath.Join(root, "zellij", "plugins", "zjstatus.wasm")); err != nil {
		t.Fatalf("zjstatus.wasm: %v", err)
	}
	star, err := os.ReadFile(filepath.Join(root, "starship", "starship.toml"))
	if err != nil || len(star) == 0 {
		t.Fatalf("starship.toml %v", err)
	}
	zsh, err := os.ReadFile(filepath.Join(root, "shells", "zsh", "zshrc"))
	if err != nil || !strings.Contains(string(zsh), "dotfiles:") {
		t.Fatalf("zshrc %v %q", err, zsh)
	}

	home := t.TempDir()
	src := filepath.Join(root, "zellij", "config.kdl")
	dst := filepath.Join(home, ".config", "zellij", "config.kdl")
	res, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, time.Now(), home)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "created" {
		t.Fatalf("action %s", res.Action)
	}
	body, err := os.ReadFile(dst)
	if err != nil || string(body) != string(got) {
		t.Fatalf("copied %d bytes %v", len(body), err)
	}
	info, err := os.Stat(dst)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("file mode %v", info)
	}
	dir, err := os.Stat(filepath.Dir(dst))
	if err != nil || dir.Mode().Perm() != 0o755 {
		t.Fatalf("dir mode %v", dir)
	}
}

func TestBackupOnDiff(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(src, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 1, 2, 3, 0, time.UTC)
	res, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, now, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "backed_up" {
		t.Fatalf("action %s", res.Action)
	}
	bak := dst + ".bak.20261110T010203Z"
	b, err := os.ReadFile(bak)
	if err != nil || string(b) != "old\n" {
		t.Fatalf("bak %q %v", b, err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "new\n" {
		t.Fatalf("dst %q %v", got, err)
	}
	if err := os.WriteFile(src, []byte("newer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, now, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bak + "-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSkipIfSame(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "config.kdl")
	body := []byte("same\n")
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(dst, past, past); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	res, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, time.Now(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "unchanged" {
		t.Fatalf("action %s", res.Action)
	}
	after, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("mtime changed %s -> %s", before.ModTime(), after.ModTime())
	}
	matches, _ := filepath.Glob(dst + ".bak.*")
	if len(matches) != 0 {
		t.Fatalf("unexpected bak %v", matches)
	}
}

func TestCopyZshOnlyIfMissing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "zshrc")
	dst := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(src, []byte(zshBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("user\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := configcopy.Deploy(src, dst, configcopy.PolicyCopyIfMissing, time.Now(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "skipped_exists" {
		t.Fatalf("action %s", res.Action)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "user\n" {
		t.Fatalf("overwrote %q", got)
	}
	os.Remove(dst)
	res, err = configcopy.Deploy(src, dst, configcopy.PolicyCopyIfMissing, time.Now(), dir)
	if err != nil || res.Action != "created" {
		t.Fatalf("%s %v", res.Action, err)
	}
	got, err = os.ReadFile(dst)
	if err != nil || string(got) != zshBody {
		t.Fatalf("created %q", got)
	}
}

func TestRefuseSymlinkDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	real := filepath.Join(dir, "real")
	dst := filepath.Join(dir, "link")
	if err := os.WriteFile(src, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, dst); err != nil {
		t.Fatal(err)
	}
	_, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, time.Now(), dir)
	if err == nil || !strings.Contains(err.Error(), "拒絕 symlink：") {
		t.Fatalf("err = %v", err)
	}
	got, _ := os.ReadFile(real)
	if string(got) != "b\n" {
		t.Fatalf("followed symlink, real = %q", got)
	}
}

func TestRefuseDestOutsideHome(t *testing.T) {
	home := t.TempDir()
	if configcopy.AllowedDest("/etc/passwd", home, "") {
		t.Fatal("/etc/passwd allowed")
	}
	outside := filepath.Join(home, "..", "passwd")
	if configcopy.AllowedDest(outside, home, "") {
		t.Fatal("../passwd allowed")
	}
	inside := filepath.Join(home, ".config", "zellij", "config.kdl")
	if !configcopy.AllowedDest(inside, home, "") {
		t.Fatal("inside home rejected")
	}
	xdg := t.TempDir()
	xdgDst := filepath.Join(xdg, "starship.toml")
	if configcopy.AllowedDest(xdgDst, home, "") {
		t.Fatal("xdg dest allowed without xdg root")
	}
	if !configcopy.AllowedDest(xdgDst, home, xdg) {
		t.Fatal("absolute xdg should allow")
	}
	if configcopy.AllowedDest(xdgDst, home, "relative") {
		t.Fatal("relative xdg should not widen the root")
	}
}

func TestRefuseParentSymlink(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(home, "link", "config.kdl")
	if configcopy.AllowedDest(dst, home, "") {
		t.Fatal("parent symlink allowed")
	}
}

func TestRefuseMissingDirUnderParentSymlink(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(home, "src")
	if err := os.WriteFile(src, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(home, "link", "zellij", "config.kdl")
	_, err := configcopy.Deploy(src, dst, configcopy.PolicyBackupReplace, time.Now(), home)
	if err == nil || !strings.Contains(err.Error(), "拒絕 symlink：") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "zellij", "config.kdl")); !os.IsNotExist(statErr) {
		t.Fatal("file created outside the root")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
