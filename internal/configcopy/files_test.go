package configcopy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/configcopy"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeRepo(t *testing.T) string {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "starship", "starship.toml"), "theme\n")
	writeFile(t, filepath.Join(repo, "shells", "starship", "starship.toml"), "stub\n")
	writeFile(t, filepath.Join(repo, "zellij", "config.kdl"), "cfg\n")
	writeFile(t, filepath.Join(repo, "zellij", "layouts", "default.kdl"), "layout\n")
	writeFile(t, filepath.Join(repo, "zellij", "plugins", "zjstatus.wasm"), "wasm")
	writeFile(t, filepath.Join(repo, "zellij", "plugins", ".gitkeep"), "")
	writeFile(t, filepath.Join(repo, "shells", "zsh", "zshrc"), "zsh\n")
	return repo
}

func TestFilesFor(t *testing.T) {
	repo := fakeRepo(t)
	home := "/home/u"
	cases := []struct {
		id, xdg string
		want    []string // src-rel -> dst
	}{
		{"starship", "", []string{"starship/starship.toml -> /home/u/.config/starship.toml"}},
		{"starship", "/x", []string{"starship/starship.toml -> /x/starship.toml"}},
		{"starship", "rel", []string{"starship/starship.toml -> /home/u/.config/starship.toml"}},
		{"zellij", "", []string{
			"zellij/config.kdl -> /home/u/.config/zellij/config.kdl",
			"zellij/layouts/default.kdl -> /home/u/.config/zellij/layouts/default.kdl",
			"zellij/plugins/zjstatus.wasm -> /home/u/.config/zellij/plugins/zjstatus.wasm",
		}},
		{"zsh", "", []string{"shells/zsh/zshrc -> /home/u/.zshrc"}},
		{"git", "", nil},
	}
	for _, c := range cases {
		files, err := configcopy.FilesFor(c.id, repo, home, c.xdg)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range files {
			rel, _ := filepath.Rel(repo, f.Src)
			got = append(got, filepath.ToSlash(rel)+" -> "+f.Dst)
		}
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s xdg=%q:\n%s", c.id, c.xdg, strings.Join(got, "\n"))
		}
	}
	if configcopy.HasConfig("git") || !configcopy.HasConfig("zellij") {
		t.Error("HasConfig")
	}
	if _, err := configcopy.FilesFor("zsh", "", home, ""); err == nil {
		t.Error("empty repo accepted")
	}
}

func TestFilesForStarshipFallback(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "shells", "starship", "starship.toml"), "stub\n")
	files, _ := configcopy.FilesFor("starship", repo, "/h", "")
	if len(files) != 1 || !strings.HasSuffix(files[0].Src, filepath.Join("shells", "starship", "starship.toml")) {
		t.Fatalf("%+v", files)
	}
}

func TestUndeploy(t *testing.T) {
	now := time.Date(2026, 11, 17, 6, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		policy  configcopy.Policy
		have    *string
		action  string
		dstGone bool
		backup  bool
	}{
		{"same bytes", configcopy.PolicyBackupReplace, strp("repo\n"), "removed", true, false},
		{"edited", configcopy.PolicyBackupReplace, strp("mine\n"), "backed_up", true, true},
		{"edited zshrc", configcopy.PolicyCopyIfMissing, strp("mine\n"), "kept_modified", false, false},
		{"same zshrc", configcopy.PolicyCopyIfMissing, strp("repo\n"), "removed", true, false},
		{"absent", configcopy.PolicyBackupReplace, nil, "absent", true, false},
	}
	for _, c := range cases {
		home := t.TempDir()
		src := filepath.Join(t.TempDir(), "src")
		writeFile(t, src, "repo\n")
		dst := filepath.Join(home, ".config", "zellij", "layouts", "x.kdl")
		if c.have != nil {
			writeFile(t, dst, *c.have)
		}
		res, err := configcopy.Undeploy(configcopy.File{Src: src, Dst: dst, Policy: c.policy}, now, home, filepath.Join(home, ".config"))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Action != c.action {
			t.Errorf("%s: action %s", c.name, res.Action)
		}
		_, err = os.Stat(dst)
		if gone := os.IsNotExist(err); gone != c.dstGone {
			t.Errorf("%s: gone=%v", c.name, gone)
		}
		bak := dst + ".bak.20261117T060000Z"
		if _, err := os.Stat(bak); (err == nil) != c.backup {
			t.Errorf("%s: backup %v", c.name, err)
		}
		if c.action == "removed" {
			if _, err := os.Stat(filepath.Join(home, ".config", "zellij")); !os.IsNotExist(err) {
				t.Errorf("%s: empty dirs not pruned", c.name)
			}
			if _, err := os.Stat(filepath.Join(home, ".config")); err != nil {
				t.Errorf("%s: pruned past the config root", c.name)
			}
		}
	}
}

func TestUndeployRefusesSymlinkAndOutside(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "src")
	writeFile(t, src, "x")
	link := filepath.Join(home, "starship.toml")
	if err := os.Symlink(src, link); err != nil {
		t.Skip(err)
	}
	if _, err := configcopy.Undeploy(configcopy.File{Src: src, Dst: link}, time.Now(), home); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("symlink target touched")
	}
	if _, err := configcopy.Undeploy(configcopy.File{Src: src, Dst: src}, time.Now(), home); err == nil {
		t.Fatal("outside root accepted")
	}
}

func strp(s string) *string { return &s }
