package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
)

func fedoraInfo() platform.Info {
	return platform.Info{
		OS:      platform.OSInfo{GOOS: "linux", SystemText: "Fedora Linux 45 (WSL) [GOOS=linux]"},
		Arch:    platform.ArchInfo{Family: "x86_64", GOARCH: "amd64", UnameM: "x86_64"},
		Manager: "dnf",
	}
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestToggle(t *testing.T) {
	m := New(fedoraInfo())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm := next.(model)
	if mm.selected["git"] {
		t.Fatal("space should clear git")
	}
	if !mm.itemByID("git").Default {
		t.Fatal("default marker changed")
	}
	view := render(mm, false)
	if !strings.Contains(view, "> * [ ] git") {
		t.Fatalf("%s", view)
	}
	if strings.Contains(view, "> * [x] git") {
		t.Fatal(view)
	}
}

func TestToggleAllInCategory(t *testing.T) {
	m := New(fedoraInfo())
	next, _ := m.Update(keyRune('a'))
	mm := next.(model)
	for _, c := range mm.cats {
		if c.ID != "basics" {
			continue
		}
		for _, it := range c.Items {
			if mm.selected[it.ID] {
				t.Fatalf("%s still selected", it.ID)
			}
			if it.Default && !strings.Contains(render(mm, false), "* [ ] "+it.Name) && it.ID == "git" {
				t.Fatal(render(mm, false))
			}
		}
	}
	if mm.selected["zsh"] {
		t.Fatal("zsh changed")
	}
	next, _ = mm.Update(keyRune('a'))
	mm = next.(model)
	for _, id := range []catalog.ID{"git", "curl", "wget"} {
		if !mm.selected[id] {
			t.Fatalf("%s not rechecked", id)
		}
	}
}

func TestEnterWithEmptySelection(t *testing.T) {
	m := New(fedoraInfo())
	next, _ := m.Update(keyRune('A'))
	next, _ = next.Update(keyRune('A'))
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := next.(model)
	if mm.phase != phaseBrowsing {
		t.Fatalf("phase %d", mm.phase)
	}
	if mm.status != "尚未勾選任何項目" {
		t.Fatalf("status %q", mm.status)
	}
	if !strings.Contains(render(mm, false), "尚未勾選任何項目") {
		t.Fatal(render(mm, false))
	}
}

func TestKeymapHelpVisible(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	view := render(m, false)
	browsing := "操作  ↑/k 上一項  ↓/j 下一項  space 勾選/取消  enter 安裝已勾選  a 切換本類  A 切換全部  d 只顯示計畫  g 頂端  G 底端  q 離開"
	if !strings.Contains(view, browsing) {
		t.Fatalf("browsing help missing\n%s", view)
	}
	if browsingHelp(m.keys) != browsing {
		t.Fatalf("built %q", browsingHelp(m.keys))
	}
	m.phase = phaseConfirm
	if !strings.Contains(render(m, false), "操作  y 開始  n/esc 返回") {
		t.Fatal(render(m, false))
	}
	m.phase = phaseInstalling
	if !strings.Contains(render(m, false), "操作  ctrl+c 取消") {
		t.Fatal(render(m, false))
	}
	m.phase = phaseReport
	m.results = []install.ItemResult{{ID: "git", Status: "ok", Note: "note"}}
	rep := render(m, false)
	if !strings.Contains(rep, "操作  任意鍵返回  q 離開") || !strings.Contains(rep, "git  ok  note") {
		t.Fatalf("%s", rep)
	}
	m.phase = phaseFailed
	if !strings.Contains(render(m, false), browsing) {
		t.Fatal("failed help")
	}
}

func TestReportQQuitsOtherKeyReturns(t *testing.T) {
	m := New(fedoraInfo())
	m.phase = phaseReport
	m.selected["git"] = false
	snap := copySel(m.selected)
	_, cmd := m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("ctrl+c should stay")
		}
	}
	mm := next.(model)
	if mm.phase != phaseBrowsing {
		t.Fatalf("phase %d", mm.phase)
	}
	if !sameSel(mm.selected, snap) {
		t.Fatal("selection changed")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = next.(model)
	if mm.phase != phaseBrowsing || !sameSel(mm.selected, snap) {
		t.Fatal("enter should return without changing selection")
	}
}

func TestHomeKeys(t *testing.T) {
	m := New(fedoraInfo())
	m.phase = phaseHome
	m.homeUser = "shuhi"
	m.homeHost = "dotfiles"
	snap := copySel(m.selected)
	cursor := m.cursor

	next, _ := m.Update(keyRune('j'))
	mm := next.(model)
	if mm.homeIndex != 1 || mm.cursor != cursor || mm.scroll != 0 || !sameSel(mm.selected, snap) {
		t.Fatal("j")
	}
	next, _ = mm.Update(keyRune('j'))
	if next.(model).homeIndex != 1 {
		t.Fatal("wrap down")
	}
	next, _ = mm.Update(keyRune('k'))
	mm = next.(model)
	if mm.homeIndex != 0 || mm.cursor != cursor || mm.scroll != 0 || !sameSel(mm.selected, snap) {
		t.Fatal("k")
	}
	next, _ = mm.Update(keyRune('k'))
	if next.(model).homeIndex != 0 {
		t.Fatal("wrap up")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	if cmd != nil || mm.phase != phaseHome || !sameSel(mm.selected, snap) {
		t.Fatal("space")
	}

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter command")
	}
	mm = next.(model)
	if mm.phase != phaseBrowsing || !strings.Contains(render(mm, false), "基礎") {
		t.Fatal(render(mm, false))
	}

	down, _ := m.Update(keyRune('j'))
	next, cmd = down.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("uninstall command")
	}
	um := next.(model)
	if um.phase != phaseUnavailable || um.homeIndex != 1 {
		t.Fatal("unavailable")
	}
	uv := render(um, false)
	if !strings.Contains(uv, "此動作不可用") || !strings.Contains(uv, "不移除發行版套件，也不刪除檔案。") {
		t.Fatal(uv)
	}
	for _, bad := range []string{"apt-get", "dnf", "brew uninstall"} {
		if strings.Contains(uv, bad) {
			t.Fatal(bad)
		}
	}
	back, cmd := um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || back.(model).phase != phaseHome || back.(model).homeIndex != 1 {
		t.Fatal("enter back")
	}
	back, cmd = um.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || back.(model).phase != phaseHome || back.(model).homeIndex != 1 {
		t.Fatal("esc back")
	}

	_, cmd = m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q quit")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c quit")
	}
}

func TestCatalogEscReturnsHome(t *testing.T) {
	m := New(fedoraInfo())
	m.cursor = 4
	m.selected["git"] = false
	snap := copySel(m.selected)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("cmd")
	}
	mm := next.(model)
	if mm.phase != phaseHome || mm.cursor != 4 || mm.scroll != 0 || !sameSel(mm.selected, snap) {
		t.Fatalf("phase %d cursor %d", mm.phase, mm.cursor)
	}
	const browsing = "操作  ↑/k 上一項  ↓/j 下一項  space 勾選/取消  enter 安裝已勾選  a 切換本類  A 切換全部  d 只顯示計畫  g 頂端  G 底端  q 離開"
	if browsingHelp(m.keys) != browsing {
		t.Fatalf("help %q", browsingHelp(m.keys))
	}

	m.phase = phaseConfirm
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || next.(model).phase != phaseBrowsing {
		t.Fatal("confirm esc")
	}
	m.phase = phaseReport
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || next.(model).phase != phaseBrowsing {
		t.Fatal("report esc")
	}
	m.phase = phaseFailed
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || next.(model).phase != phaseFailed {
		t.Fatal("failed esc")
	}
}

func TestAskBeatsEsc(t *testing.T) {
	m := New(fedoraInfo())
	m.askName = "less"
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("cmd")
	}
	mm := next.(model)
	if mm.phase != phaseBrowsing || mm.askName != "" || mm.askTold {
		t.Fatal("esc")
	}
	m.askName = "less"
	next, cmd = m.Update(keyRune('y'))
	mm = next.(model)
	if cmd != nil || !mm.askTold || mm.askName != "less" || mm.phase != phaseBrowsing || mm.force {
		t.Fatal("y")
	}
	m.askName = "less"
	next, cmd = m.Update(keyRune('q'))
	mm = next.(model)
	if cmd != nil || mm.askName != "" || mm.phase != phaseBrowsing {
		t.Fatal("q")
	}
}

func TestDetectingEnterIgnored(t *testing.T) {
	m := New(platform.Info{})
	m.width = 80
	m.height = 40
	if m.phase != phaseDetecting {
		t.Fatal("phase")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || next.(model).phase != phaseDetecting {
		t.Fatal("enter")
	}
	moved, _ := m.Update(keyRune('j'))
	if moved.(model).homeIndex != 1 || moved.(model).phase != phaseDetecting {
		t.Fatal("j")
	}
	for _, k := range []tea.KeyMsg{keyRune('q'), tea.KeyMsg{Type: tea.KeyCtrlC}} {
		_, cmd = m.Update(k)
		if cmd == nil {
			t.Fatal("quit")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("not quit")
		}
	}
	view := render(m, false)
	lines := frameLines(view)
	if !strings.Contains(view, "Tools-install") || strings.Contains(view, "#") || strings.Contains(view, "系統:") || strings.Contains(view, browsingHelp(m.keys)) {
		t.Fatal(view)
	}
	if lines[len(lines)-1] != homeHelp {
		t.Fatalf("last %q", lines[len(lines)-1])
	}
}

func copySel(in map[catalog.ID]bool) map[catalog.ID]bool {
	out := map[catalog.ID]bool{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sameSel(a, b map[catalog.ID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
