package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

func uninstallFixture(t *testing.T, installed ...catalog.ID) model {
	t.Helper()
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	m.phase = phaseHome
	m.homeIndex = 1
	m.installed = map[catalog.ID]bool{}
	for _, id := range installed {
		m.installed[id] = true
	}
	home := t.TempDir()
	m.deps.Runner.Home = home
	files := map[string]bool{
		filepath.Join(home, ".local", "bin", "starship"): true,
		"/usr/bin/python3": true,
		"/usr/bin/jq":      true,
	}
	m.deps.Exists = func(p string) bool { return files[p] }
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(model)
}

func press(t *testing.T, m model, keys ...tea.KeyMsg) model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(model)
	}
	return m
}

var (
	kEnter = tea.KeyMsg{Type: tea.KeyEnter}
	kEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	kSpace = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
)

func TestUninstallListsOnlyInstalledRemovable(t *testing.T) {
	m := uninstallFixture(t, "git", "vim", "python", "jq", "starship", "zellij")
	if m.phase != phaseUninstall || m.mode != modeUninstall {
		t.Fatalf("phase %d mode %d", m.phase, m.mode)
	}
	got := m.removableIDs()
	want := []catalog.ID{"starship", "jq", "zellij", "python"}
	if len(got) != len(want) {
		t.Fatalf("removable %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("removable %v", got)
		}
	}
	m.width = 100 // descriptions show from 100 columns
	view := render(m, false)
	m.width = 80
	for _, s := range []string{"starship", "jq", "zellij", "python", "移除前會先警告", uninstallNote, "已勾選移除 0", "esc/b 返回首頁", "移除已勾選"} {
		if !strings.Contains(view, s) {
			t.Errorf("missing %q\n%s", s, view)
		}
	}
	for _, s := range []string{"  git", "  vim", "\n  基礎\n", "(已安裝)"} {
		if strings.Contains(view, s) {
			t.Errorf("unexpected %q\n%s", s, view)
		}
	}
	styled := renderStyled(m, testRenderer())
	for _, ln := range frameLines(styled) {
		if ansi.StringWidth(ln) != 80 {
			t.Fatalf("width %d %q", ansi.StringWidth(ln), ln)
		}
	}
}

func TestUninstallEmptyList(t *testing.T) {
	m := uninstallFixture(t, "git", "curl")
	if m.status != uninstallEmpty || len(m.removableIDs()) != 0 {
		t.Fatalf("status %q", m.status)
	}
	m = press(t, m, kSpace, kEnter)
	if m.phase != phaseUninstall || m.status != "尚未勾選任何項目" {
		t.Fatalf("phase %d status %q", m.phase, m.status)
	}
}

func TestUninstallSelectConfirmAndBack(t *testing.T) {
	m := uninstallFixture(t, "starship", "jq")
	m = press(t, m, kEnter)
	if m.phase != phaseUninstall || m.status != "尚未勾選任何項目" {
		t.Fatal("enter without selection")
	}
	m = press(t, m, kSpace) // starship
	if !m.uselected["starship"] || m.uselected["jq"] {
		t.Fatalf("selected %v", m.uselected)
	}
	m = press(t, m, kEnter)
	if m.phase != phaseConfirm || m.mode != modeUninstall {
		t.Fatalf("phase %d", m.phase)
	}
	var titles []string
	for _, st := range m.steps {
		titles = append(titles, st.Title)
	}
	if strings.Join(titles, ",") != "release-remove starship,shell-rc starship,config starship" {
		t.Fatalf("titles %v", titles)
	}
	view := render(m, false)
	if !strings.Contains(view, "將移除下列項目") || !strings.Contains(view, "release-remove starship") {
		t.Fatal(view)
	}
	m = press(t, m, keyRune('n'))
	if m.phase != phaseUninstall || !m.uselected["starship"] {
		t.Fatalf("n back: %d", m.phase)
	}
	// a / A toggle
	m = press(t, m, keyRune('A'))
	if !m.uselected["jq"] || !m.uselected["starship"] {
		t.Fatalf("A on %v", m.uselected)
	}
	m = press(t, m, keyRune('A'))
	if m.uselected["jq"] || m.uselected["starship"] {
		t.Fatalf("A off %v", m.uselected)
	}
	m = press(t, m, keyRune('j'), keyRune('j'), keyRune('j'))
	if m.ucursor != 1 {
		t.Fatalf("cursor clamps at the last row: %d", m.ucursor)
	}
	m = press(t, m, keyRune('b'))
	if m.phase != phaseHome || m.mode != modeInstall {
		t.Fatalf("b home %d", m.phase)
	}
}

func TestUninstallPythonNeedsWarning(t *testing.T) {
	m := uninstallFixture(t, "python")
	m = press(t, m, kSpace, kEnter)
	if m.phase != phasePythonWarn {
		t.Fatalf("phase %d", m.phase)
	}
	view := render(m, false)
	if !strings.Contains(view, uninstall.PythonWarning) || !strings.Contains(view, pythonWarnHelp) {
		t.Fatal(view)
	}
	for _, k := range []tea.KeyMsg{keyRune('n'), kEsc} {
		back := press(t, m, k)
		if back.phase != phaseUninstall || len(back.usteps) != 0 {
			t.Fatalf("%s: phase %d", k.String(), back.phase)
		}
	}
	if other := press(t, m, keyRune('x'), kEnter); other.phase != phasePythonWarn {
		t.Fatal("only y confirms")
	}
	m = press(t, m, keyRune('y'))
	if m.phase != phaseConfirm || len(m.usteps) != 1 {
		t.Fatalf("phase %d steps %+v", m.phase, m.usteps)
	}
	if got := strings.Join(m.usteps[0].Argv, " "); !strings.HasSuffix(got, "dnf remove -y python3") {
		t.Fatalf("argv %q", got)
	}
}

func TestUninstallReportReturnsToList(t *testing.T) {
	m := uninstallFixture(t, "jq")
	m.phase = phaseInstalling
	m.runID = 7
	next, cmd := m.Update(bridgeMsg{runID: 7, kind: kindDone, code: 0, results: []install.ItemResult{{ID: "jq", Status: uninstall.StatusRemoved}}})
	m = next.(model)
	if m.phase != phaseReport || cmd == nil {
		t.Fatalf("phase %d", m.phase)
	}
	if !strings.Contains(render(m, false), "jq  removed") {
		t.Fatal(render(m, false))
	}
	m = press(t, m, keyRune('x'))
	if m.phase != phaseUninstall || m.status != uninstallNote {
		t.Fatalf("phase %d", m.phase)
	}
}

func TestBrowseBackKey(t *testing.T) {
	for _, k := range []tea.KeyMsg{kEsc, keyRune('b')} {
		m := New(fedoraInfo())
		m.cursor = 3
		next, cmd := m.Update(k)
		mm := next.(model)
		if cmd != nil || mm.phase != phaseHome || mm.cursor != 3 {
			t.Fatalf("%s: phase %d", k.String(), mm.phase)
		}
	}
	m := New(fedoraInfo())
	m.width = 160
	if !strings.Contains(render(m, false), "esc/b 返回首頁") {
		t.Fatal("back key not shown in the footer")
	}
}

func TestCatalogHeadersFollowTitleGradient(t *testing.T) {
	m := New(fedoraInfo())
	m.height = 60
	view := renderStyled(m, testRenderer())
	cases := map[string]string{
		"基礎": "1;38;2;240;173;160", // #F0ADA0
		"編譯": "1;38;2;158;226;240", // #9EE2F0
	}
	for title, sgr := range cases {
		line := lineContaining(view, title)
		if line == "" || !strings.Contains(line, sgr) {
			t.Errorf("%s header %q", title, line)
		}
	}
	for _, old := range []string{"48;2;56;73;52", "38;2;158;208;108", "48;2;43;44;58"} {
		if strings.Contains(view, old) {
			t.Errorf("old earth color %s still used", old)
		}
	}
}
