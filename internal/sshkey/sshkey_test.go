package sshkey_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/sshkey"
	"golang.org/x/crypto/ssh"
)

func newKey(t *testing.T, passphrase string) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var blk *pem.Block
	if passphrase == "" {
		blk, err = ssh.MarshalPrivateKey(priv, "test")
	} else {
		blk, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "test", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(blk)
}

func TestLayout(t *testing.T) {
	l := sshkey.LayoutFor("linux", "/home/u")
	if l.KeyDir != "/home/u/.ssh/keys" || l.Identity("gh") != "~/.ssh/keys/gh" || l.ConfigPath() != "/home/u/.ssh/config" {
		t.Fatalf("%+v", l)
	}
	if d := sshkey.LayoutFor("darwin", "/Users/u"); d.KeyDir != "/Users/u/.ssh/keys" {
		t.Fatalf("%+v", d)
	}
	w := sshkey.LayoutFor("windows", "/home/u")
	if w.KeyDir != filepath.Join("/home/u", ".ssh") || w.Identity("gh") != "~/.ssh/gh" {
		t.Fatalf("%+v", w)
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"github", "id_ed25519", "work-key.2024", "A1"} {
		if err := sshkey.ValidName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a/b", "../x", ".hidden", "-x", "a b", "key.pub", "config", "known_hosts", "authorized_keys", "x\\y", strings.Repeat("a", 65), "鑰匙"} {
		if sshkey.ValidName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestListKeys(t *testing.T) {
	dir := t.TempDir()
	key := newKey(t, "")
	write := func(name string, body []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("github", key)
	write("github.pub", []byte("ssh-ed25519 AAAA test\n"))
	write("known_hosts", []byte("github.com ssh-ed25519 AAAA\n"))
	write("config", []byte("Host *\n"))
	write("notes.txt", []byte("hello\n"))
	write("old_rsa", []byte("-----BEGIN RSA PRIVATE KEY-----\nxx\n-----END RSA PRIVATE KEY-----\n"))
	os.Mkdir(filepath.Join(dir, "sub"), 0o700)
	got, err := sshkey.ListKeys(dir)
	if err != nil || strings.Join(got, ",") != "github,old_rsa" {
		t.Fatalf("%v %v", got, err)
	}
	if got, err := sshkey.ListKeys(filepath.Join(dir, "missing")); err != nil || got != nil {
		t.Fatalf("missing dir: %v %v", got, err)
	}
}

func TestSaveKey(t *testing.T) {
	home := t.TempDir()
	l := sshkey.LayoutFor("linux", home)
	key := newKey(t, "")
	crlf := []byte("\r\n  " + strings.ReplaceAll(strings.TrimSpace(string(key)), "\n", "\r\n") + "  \r\n")
	path, err := l.SaveKey("github", crlf)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(home, ".ssh", "keys", "github") {
		t.Fatal(path)
	}
	b, _ := os.ReadFile(path)
	if string(b) != string(key) || !strings.HasSuffix(string(b), "-----\n") {
		t.Fatalf("saved body differs from the normalised key")
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{path: 0o600, l.KeyDir: 0o700, l.SSHDir: 0o700} {
			if info, _ := os.Stat(p); info.Mode().Perm() != want {
				t.Errorf("%s mode %v want %v", p, info.Mode().Perm(), want)
			}
		}
	}
	if _, err := l.SaveKey("github", newKey(t, "")); err == nil || !strings.Contains(err.Error(), "換一個名稱") {
		t.Fatalf("duplicate: %v", err)
	}
	if b2, _ := os.ReadFile(path); string(b2) != string(key) {
		t.Fatal("duplicate overwrote the key")
	}
	if _, err := l.SaveKey("locked", newKey(t, "secret")); err != nil {
		t.Fatalf("passphrase key: %v", err)
	}
	for name, body := range map[string][]byte{
		"pub":     []byte("ssh-ed25519 AAAA test"),
		"garbage": []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nnot base64\n-----END OPENSSH PRIVATE KEY-----"),
		"empty":   nil,
	} {
		_, err := l.SaveKey(name, body)
		if err == nil {
			t.Errorf("%s accepted", name)
		} else if strings.Contains(err.Error(), "BEGIN OPENSSH") && name == "garbage" {
			t.Errorf("error leaks key text: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(l.KeyDir, name)); statErr == nil {
			t.Errorf("%s written", name)
		}
	}
	if _, err := l.SaveKey("../evil", key); err == nil {
		t.Fatal("path name accepted")
	}
}

const block = "# >>> dotfiles:github >>>\nHost github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/keys/github\n# <<< dotfiles:github <<<\n"

func TestApplyConfig(t *testing.T) {
	home := t.TempDir()
	l := sshkey.LayoutFor("linux", home)
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	res, err := l.Apply(l.Identity("github"), now)
	if err != nil || !res.Changed || res.Backup != "" {
		t.Fatalf("%+v %v", res, err)
	}
	b, _ := os.ReadFile(l.ConfigPath())
	if string(b) != block {
		t.Fatalf("%q", b)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(l.ConfigPath()); info.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", info.Mode().Perm())
		}
	}
	res, _ = l.Apply(l.Identity("github"), now)
	if res.Changed {
		t.Fatal("second apply rewrote the file")
	}
	if sshkey.Current(l.ConfigPath()) != "~/.ssh/keys/github" {
		t.Fatal("Current")
	}
	l.Apply(l.Identity("work"), now)
	b, _ = os.ReadFile(l.ConfigPath())
	if strings.Count(string(b), sshkey.BlockBegin) != 1 || !strings.Contains(string(b), "IdentityFile ~/.ssh/keys/work") {
		t.Fatalf("%q", b)
	}
}

func TestApplyKeepsUserConfigAndBacksUp(t *testing.T) {
	home := t.TempDir()
	l := sshkey.LayoutFor("linux", home)
	os.MkdirAll(l.SSHDir, 0o700)
	user := "Host github.com\n  IdentityFile ~/.ssh/old\n\nHost box\n  User me\n"
	os.WriteFile(l.ConfigPath(), []byte(user), 0o644)
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	res, err := l.Apply(l.Identity("github"), now)
	if err != nil || !res.Unmanaged || res.Backup != l.ConfigPath()+".bak.20261117T080000Z" {
		t.Fatalf("%+v %v", res, err)
	}
	if bak, _ := os.ReadFile(res.Backup); string(bak) != user {
		t.Fatal("backup differs")
	}
	b, _ := os.ReadFile(l.ConfigPath())
	if string(b) != block+"\n"+user {
		t.Fatalf("managed block must come first:\n%s", b)
	}
	res, _ = l.Apply(l.Identity("work"), now.Add(time.Hour))
	if res.Backup != "" {
		t.Fatal("backed up again although the block was already there")
	}
	removed, err := sshkey.RemoveBlock(l.ConfigPath())
	b, _ = os.ReadFile(l.ConfigPath())
	if !removed || err != nil || string(b) != user {
		t.Fatalf("remove: %v %v %q", removed, err, b)
	}
	if removed, _ := sshkey.RemoveBlock(l.ConfigPath()); removed {
		t.Fatal("second remove")
	}
	if removed, err := sshkey.RemoveBlock(filepath.Join(home, "none")); removed || err != nil {
		t.Fatal("missing file")
	}
}
