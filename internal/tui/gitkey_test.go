package tui

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"golang.org/x/crypto/ssh"
)

func testKey(t *testing.T) string {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	blk, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(blk))
}

func gitModel(t *testing.T, goos string) (model, string) {
	t.Helper()
	m := New(fedoraInfo())
	m.width, m.height = 120, 40
	home := t.TempDir()
	m.deps.Runner.Home = home
	m.deps.GOOS = goos
	m.deps.Now = func() time.Time { return time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC) }
	m.deps.Detect = func(string) map[catalog.ID]bool { return nil }
	m.mode = modeInstall
	m.finishRun([]install.ItemResult{{ID: "git", Status: "satisfied"}, {ID: "jq", Status: "ok"}}, 0)
	return m, home
}

func TestGitKeyPasteNewKey(t *testing.T) {
	m, home := gitModel(t, "linux")
	if !m.gitPending || !strings.Contains(render(m, false), "任意鍵 設定 GitHub SSH 鑰匙") {
		t.Fatalf("report does not offer the key setup\n%s", render(m, false))
	}
	m = press(t, m, keyRune('x'))
	if m.phase != phaseGitKey || m.gitStage != gitAdd {
		t.Fatalf("phase %d stage %d", m.phase, m.gitStage)
	}
	key := testKey(t)
	secret := strings.Split(key, "\n")[1]
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key), Paste: true})
	view := render(m, false)
	if strings.Contains(view, secret) || !strings.Contains(view, "-----BEGIN OPENSSH PRIVATE KEY-----") || !strings.Contains(view, "已貼上") {
		t.Fatalf("key page shows key body or no state\n%s", view)
	}
	// enter in the key field is a newline, not a save
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.gitStage != gitAdd {
		t.Fatal("enter in the key field saved")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("../bad")}, tea.KeyMsg{Type: tea.KeyEnter})
	if m.gitStage != gitAdd || m.gitNote == "" {
		t.Fatalf("bad name accepted: %q", m.gitNote)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("github")}, tea.KeyMsg{Type: tea.KeyEnter})
	if m.gitStage != gitDone || m.gitBuf != nil {
		t.Fatalf("stage %d note %q", m.gitStage, m.gitNote)
	}
	saved, err := os.ReadFile(filepath.Join(home, ".ssh", "keys", "github"))
	if err != nil || string(saved) != key {
		t.Fatalf("saved key %v", err)
	}
	cfg, _ := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(string(cfg), "Host github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/keys/github\n") {
		t.Fatalf("config %q", cfg)
	}
	if v := render(m, false); strings.Contains(v, secret) || !strings.Contains(v, "~/.ssh/keys/github") {
		t.Fatal(v)
	}
	m = press(t, m, keyRune('z'))
	if m.phase != phaseHome {
		t.Fatalf("phase %d", m.phase)
	}
}

func TestGitKeyPickExisting(t *testing.T) {
	m, home := gitModel(t, "linux")
	dir := filepath.Join(home, ".ssh", "keys")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "alpha"), []byte(testKey(t)), 0o600)
	os.WriteFile(filepath.Join(dir, "beta"), []byte(testKey(t)), 0o600)
	os.WriteFile(filepath.Join(dir, "beta.pub"), []byte("ssh-ed25519 AAAA\n"), 0o644)
	m = press(t, m, keyRune('x'))
	if m.gitStage != gitPick || strings.Join(m.gitKeys, ",") != "alpha,beta" {
		t.Fatalf("stage %d keys %v", m.gitStage, m.gitKeys)
	}
	view := render(m, false)
	if !strings.Contains(view, "> alpha") || !strings.Contains(view, gitAddNew) || strings.Contains(view, "beta.pub") {
		t.Fatal(view)
	}
	m = press(t, m, keyRune('j'), tea.KeyMsg{Type: tea.KeyEnter})
	cfg, _ := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if m.gitStage != gitDone || !strings.Contains(string(cfg), "IdentityFile ~/.ssh/keys/beta") {
		t.Fatalf("%q", cfg)
	}
	// "add new" from the list, then esc goes back to the list
	m, _ = gitModel(t, "linux")
	m.deps.Runner.Home = home
	m = press(t, m, keyRune('x'), keyRune('j'), keyRune('j'), tea.KeyMsg{Type: tea.KeyEnter})
	if m.gitStage != gitAdd {
		t.Fatalf("stage %d", m.gitStage)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.gitStage != gitPick {
		t.Fatal("esc from add did not return to the list")
	}
}

func TestGitKeyEscSkips(t *testing.T) {
	m, home := gitModel(t, "linux")
	m = press(t, m, keyRune('x'), tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseHome {
		t.Fatalf("phase %d", m.phase)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "config")); err == nil {
		t.Fatal("skip wrote a config")
	}
}

func TestGitKeyNotOfferedWithoutGit(t *testing.T) {
	m := New(fedoraInfo())
	m.deps.Detect = func(string) map[catalog.ID]bool { return nil }
	m.mode = modeInstall
	m.finishRun([]install.ItemResult{{ID: "git", Status: "failed"}, {ID: "jq", Status: "ok"}}, 1)
	if m.gitPending {
		t.Fatal("offered after git failed")
	}
	m = press(t, m, keyRune('x'))
	if m.phase != phaseHome {
		t.Fatalf("phase %d", m.phase)
	}
	m.mode = modeUninstall
	m.finishRun([]install.ItemResult{{ID: "git", Status: "ok"}}, 0)
	if m.gitPending {
		t.Fatal("offered after an uninstall run")
	}
}

func TestGitKeyWindowsLayout(t *testing.T) {
	m, home := gitModel(t, "windows")
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte(testKey(t)), 0o600)
	os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte("x\n"), 0o600)
	m = press(t, m, keyRune('x'))
	if strings.Join(m.gitKeys, ",") != "id_ed25519" {
		t.Fatalf("%v", m.gitKeys)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	cfg, _ := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(string(cfg), "IdentityFile ~/.ssh/id_ed25519\n") {
		t.Fatalf("%q", cfg)
	}
}

// sudo runs give files back to the user after an install and a key save.
func TestFixOwnerCalledAfterRunAndKeySave(t *testing.T) {
	m, home := gitModel(t, "linux")
	_ = home
	calls := 0
	m.deps.FixOwner = func() { calls++ }
	m.finishRun([]install.ItemResult{{ID: "git", Status: "ok"}}, 0)
	if calls != 1 {
		t.Fatalf("finishRun calls %d", calls)
	}
	m = press(t, m, keyRune('x'))
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(testKey(t)), Paste: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyCtrlU}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("github")}, tea.KeyMsg{Type: tea.KeyEnter})
	if m.gitStage != gitDone {
		t.Fatalf("stage %d note %q", m.gitStage, m.gitNote)
	}
	if calls < 2 {
		t.Fatalf("key save did not fix owners (calls %d)", calls)
	}
}
