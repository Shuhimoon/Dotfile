package tui

import (
	"strings"
	"testing"
)

const whiteFG = "38;2;255;255;255"

func TestInstallDryPlanIsWhiteAndReturnsToList(t *testing.T) {
	m := New(fedoraInfo())
	m.width, m.height = 160, 60
	m = toggleOnly(t, m, "jq", "zig")
	m = press(t, m, keyRune('d'))
	if m.phase != phaseReport || !m.dryRun {
		t.Fatalf("phase %d dry %v", m.phase, m.dryRun)
	}
	view := render(m, false)
	for _, s := range []string{"只顯示計畫，沒有執行", "install jq  sudo dnf install -y jq", "install zig  sudo dnf install -y zig（找不到套件時改用：下載 ziglang.org/zig 到 ~/.local/bin）", "任意鍵 回安裝畫面"} {
		if !strings.Contains(view, s) {
			t.Errorf("missing %q\n%s", s, view)
		}
	}
	styled := renderStyled(m, testRenderer())
	found := false
	for _, ln := range strings.Split(styled, "\n") {
		if strings.Contains(ln, "install jq") {
			found = true
			if !strings.Contains(ln, whiteFG) {
				t.Fatalf("plan line not white: %q", ln)
			}
		}
	}
	if !found {
		t.Fatal("plan line missing from styled view")
	}
	if strings.Contains(styled, "48;2;") {
		t.Fatal("background colour on the plan page")
	}
	m = press(t, m, keyRune('x'))
	if m.phase != phaseBrowsing || m.dryRun || !m.selected["jq"] || !m.selected["zig"] {
		t.Fatalf("phase %d dry %v selected %v", m.phase, m.dryRun, m.selectedIDs())
	}
}

func TestUninstallDryPlanRunsNothing(t *testing.T) {
	m := uninstallFixture(t, "jq", "starship")
	m.width, m.height = 160, 60
	rec := &recordExec{}
	m.deps.Runner.Exec = rec
	m = press(t, m, keyRune('A'), keyRune('d'))
	if m.phase != phaseReport || !m.dryRun || m.mode != modeUninstall {
		t.Fatalf("phase %d dry %v mode %d", m.phase, m.dryRun, m.mode)
	}
	view := render(m, false)
	for _, s := range []string{"remove jq  sudo dnf remove -y jq", "release-remove starship  刪除 ~/.local/bin/starship", "dotfiles:starship", "任意鍵 回移除畫面"} {
		if !strings.Contains(view, s) {
			t.Errorf("missing %q\n%s", s, view)
		}
	}
	styled := renderStyled(m, testRenderer())
	found := false
	for _, ln := range strings.Split(styled, "\n") {
		if strings.Contains(ln, "remove jq") {
			found = true
			if !strings.Contains(ln, whiteFG) {
				t.Fatalf("plan line not white: %q", ln)
			}
		}
	}
	if !found {
		t.Fatal("plan line missing from styled view")
	}
	if len(rec.calls()) != 0 {
		t.Fatalf("dry run executed %v", rec.calls())
	}
	m = press(t, m, keyRune('q'))
	if m.phase != phaseUninstall || m.dryRun || !m.uselected["jq"] || !m.uselected["starship"] {
		t.Fatalf("phase %d dry %v", m.phase, m.dryRun)
	}
	if !strings.Contains(uninstallHelp(m.keys), "d 只顯示計畫") {
		t.Fatal("uninstall footer lacks d")
	}
}

func toggleOnly(t *testing.T, m model, ids ...string) model {
	t.Helper()
	for _, id := range m.flat {
		m.selected[id] = false
	}
	for _, want := range ids {
		for _, id := range m.flat {
			if string(id) == want {
				m.selected[id] = true
			}
		}
	}
	return m
}
