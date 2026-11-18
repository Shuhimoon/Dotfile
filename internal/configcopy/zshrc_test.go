package configcopy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

func TestZshrcPolicy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "zshrc")
	dst := filepath.Join(dir, "home", ".zshrc")
	tmpl := "# template\n"
	if err := os.WriteFile(src, []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	res, err := configcopy.Deploy(src, dst, configcopy.PolicyZshrc, now, dir)
	if err != nil || res.Action != "created" {
		t.Fatalf("create %+v %v", res, err)
	}
	// Template + managed block is the same file: left alone.
	withFzf, _, _ := shellrc.Ensure([]byte(tmpl), shellrc.Zsh, []shellrc.BlockID{shellrc.BlockFzf})
	_ = os.WriteFile(dst, withFzf, 0o644)
	res, err = configcopy.Deploy(src, dst, configcopy.PolicyZshrc, now, dir)
	if err != nil || res.Action != "unchanged" {
		t.Fatalf("unchanged %+v %v", res, err)
	}
	f := configcopy.File{Src: src, Dst: dst, Policy: configcopy.PolicyZshrc}
	if res, err := configcopy.Undeploy(f, now, dir); err != nil || res.Action != "removed" {
		t.Fatalf("undeploy pristine %+v %v", res, err)
	}
	// A hand-edited file: backed up and replaced, managed blocks carried over.
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	_ = os.WriteFile(dst, append([]byte("alias x=y\n"), withFzf...), 0o644)
	res, err = configcopy.Deploy(src, dst, configcopy.PolicyZshrc, now, dir)
	if err != nil || res.Action != "backed_up" {
		t.Fatalf("replace %+v %v", res, err)
	}
	got, _ := os.ReadFile(dst)
	if !strings.HasPrefix(string(got), tmpl) || !strings.Contains(string(got), shellrc.Begin(shellrc.BlockFzf)) || strings.Contains(string(got), "alias x=y") {
		t.Fatalf("new zshrc %q", got)
	}
	bak, err := os.ReadFile(dst + ".bak.20261118T000000Z")
	if err != nil || !strings.Contains(string(bak), "alias x=y") {
		t.Fatalf("backup %q %v", bak, err)
	}
	// Undeploy keeps a zshrc the user edited after deploy.
	_ = os.WriteFile(dst, []byte("mine\n"), 0o644)
	if res, err := configcopy.Undeploy(f, now, dir); err != nil || res.Action != "kept_modified" {
		t.Fatalf("undeploy edited %+v %v", res, err)
	}
	if fs, _ := configcopy.FilesFor("zsh", dir, filepath.Join(dir, "home"), ""); len(fs) != 1 || fs[0].Policy != configcopy.PolicyZshrc {
		t.Fatalf("FilesFor zsh %+v", fs)
	}
}
