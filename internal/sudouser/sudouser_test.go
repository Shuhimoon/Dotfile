package sudouser_test

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"testing"

	"github.com/shuhi/dotfiles/internal/sudouser"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func lookupHome(home string) func(string) (*user.User, error) {
	return func(name string) (*user.User, error) {
		if name != "shuhi" {
			return nil, errors.New("unknown user")
		}
		return &user.User{Username: name, Uid: "1000", Gid: "1000", HomeDir: home}, nil
	}
}

func TestDetect(t *testing.T) {
	look := lookupHome("/home/shuhi")
	if _, ok, err := sudouser.Detect(1000, env(map[string]string{"SUDO_USER": "shuhi"}), look); ok || err != nil {
		t.Fatalf("non-root run: ok=%v err=%v", ok, err)
	}
	if _, ok, err := sudouser.Detect(0, env(nil), look); ok || err != nil {
		t.Fatalf("real root keeps root home: ok=%v err=%v", ok, err)
	}
	if _, ok, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "root"}), look); ok || err != nil {
		t.Fatalf("SUDO_USER=root: ok=%v err=%v", ok, err)
	}
	tg, ok, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi", "SUDO_UID": "1001", "SUDO_GID": "1002"}), look)
	if err != nil || !ok || tg.Home != "/home/shuhi" || tg.UID != 1001 || tg.GID != 1002 || tg.Name != "shuhi" {
		t.Fatalf("sudo: %+v %v %v", tg, ok, err)
	}
	tg, ok, err = sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi"}), look)
	if err != nil || !ok || tg.UID != 1000 || tg.GID != 1000 {
		t.Fatalf("ids from passwd: %+v %v %v", tg, ok, err)
	}
	if _, _, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "ghost"}), look); err == nil {
		t.Fatal("lookup error must surface")
	}
	if _, _, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi", "SUDO_UID": "x"}), look); err == nil {
		t.Fatal("bad SUDO_UID")
	}
	if _, _, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi", "SUDO_UID": "0"}), look); err == nil {
		t.Fatal("uid 0 behind a non-root name")
	}
	if _, _, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi"}), lookupHome("/")); err == nil {
		t.Fatal("home / refused")
	}
	if _, _, err := sudouser.Detect(0, env(map[string]string{"SUDO_USER": "shuhi"}), lookupHome("relative")); err == nil {
		t.Fatal("relative home refused")
	}
}

func TestEnv(t *testing.T) {
	vars := map[string]string{"HOME": "/root", "XDG_CONFIG_HOME": "/root/.config"}
	set := func(k, v string) error { vars[k] = v; return nil }
	unset := func(k string) error { delete(vars, k); return nil }
	tg := sudouser.Target{Name: "shuhi", Home: "/home/shuhi", UID: 1000, GID: 1000}
	if err := tg.Env(set, env(vars), unset); err != nil {
		t.Fatal(err)
	}
	if vars["HOME"] != "/home/shuhi" {
		t.Fatalf("HOME %q", vars["HOME"])
	}
	if _, ok := vars["XDG_CONFIG_HOME"]; ok {
		t.Fatal("root XDG_CONFIG_HOME must be dropped")
	}
	vars["XDG_CONFIG_HOME"] = "/home/shuhi/.cfg"
	_ = tg.Env(set, env(vars), unset)
	if vars["XDG_CONFIG_HOME"] != "/home/shuhi/.cfg" {
		t.Fatal("user XDG_CONFIG_HOME kept")
	}
}

func TestRootsCoverUserFiles(t *testing.T) {
	home := t.TempDir()
	for _, f := range []string{".bashrc", ".zshrc", ".zshrc.bak.20260101T000000Z"} {
		if err := os.WriteFile(filepath.Join(home, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	for _, r := range sudouser.Roots(home) {
		got[filepath.Base(r)] = true
	}
	for _, want := range []string{".local", ".config", ".cargo", ".rustup", ".ssh", ".zsh", ".bashrc", ".zshrc", ".zshrc.bak.20260101T000000Z"} {
		if !got[want] {
			t.Fatalf("missing root %s in %v", want, got)
		}
	}
}

func TestFixerChownsOnlyRootOwned(t *testing.T) {
	home := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	rootFile := mk(".local/bin/starship")
	userFile := mk(".local/bin/mine")
	rc := mk(".zshrc")
	outside := mk("../outside-" + filepath.Base(home))
	defer os.Remove(outside)
	link := filepath.Join(home, ".config", "link")
	_ = os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	owners := map[string]int{userFile: 1000}
	var changed []string
	f := sudouser.Fixer{
		UID: 1000, GID: 1000,
		Owner: func(info fs.FileInfo) (int, bool) {
			for p, uid := range owners {
				if filepath.Base(p) == info.Name() {
					return uid, true
				}
			}
			return 0, true
		},
		Lchown: func(p string, uid, gid int) error {
			if uid != 1000 || gid != 1000 {
				t.Fatalf("ids %d:%d", uid, gid)
			}
			changed = append(changed, p)
			return nil
		},
	}
	roots := append(sudouser.Roots(home), filepath.Join(home, ".missing"))
	n, err := f.Fix(roots)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(changed)
	has := func(p string) bool {
		for _, c := range changed {
			if c == p {
				return true
			}
		}
		return false
	}
	if !has(rootFile) || !has(rc) || !has(link) || !has(filepath.Join(home, ".local", "bin")) {
		t.Fatalf("changed %v", changed)
	}
	if has(userFile) || has(outside) {
		t.Fatalf("must not touch user-owned or symlink targets: %v", changed)
	}
	if n != len(changed) {
		t.Fatalf("n=%d changed=%d", n, len(changed))
	}
}

func TestTopLevelRootOwned(t *testing.T) {
	home := t.TempDir()
	for _, n := range []string{".viminfo", "mine"} {
		if err := os.WriteFile(filepath.Join(home, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := sudouser.Fixer{Owner: func(info fs.FileInfo) (int, bool) {
		if info.Name() == ".viminfo" {
			return 0, true
		}
		return 1000, true
	}}
	got := f.TopLevel(home)
	if len(got) != 1 || filepath.Base(got[0]) != ".viminfo" {
		t.Fatalf("top level %v", got)
	}
}
