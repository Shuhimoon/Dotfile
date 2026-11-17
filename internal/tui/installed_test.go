package tui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
)

func TestMarksFromPlans(t *testing.T) {
	ids := catalog.IDs()
	if len(ids) != 35 {
		t.Fatalf("ids %d", len(ids))
	}
	brew, err := install.ResolveAll(ids, "brew")
	if err != nil {
		t.Fatal(err)
	}
	got := marksFromPlans(brew, "", func(path string) bool {
		switch path {
		case "/usr/bin/whois", "/usr/bin/make", "/sbin/traceroute":
			return true
		default:
			return false
		}
	}, false)
	if !got["whois"] || !got["make"] || !got["traceroute"] || got["git"] || got["trash"] || got["compiler"] || got["net-tools"] || got["less"] {
		t.Fatalf("%v", got)
	}
	unmarked := map[string][]catalog.ID{
		"apt":    {"zellij", "mitos", "zig"},
		"dnf":    {"starship", "zellij", "mitos"},
		"yum":    {"starship", "eza", "fastfetch", "zellij", "mitos", "zig"},
		"pacman": {"mitos"},
		"zypper": {"mitos"},
		"apk":    {"mitos"},
		"xbps":   {"mitos"},
		"brew":   {"trash", "mitos"},
	}
	wantN := map[string]int{
		"apt": 32, "dnf": 32, "yum": 29,
		"pacman": 34, "zypper": 34, "apk": 34, "xbps": 34,
		"brew": 33,
	}
	for _, mgr := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "xbps", "brew"} {
		plans, err := install.ResolveAll(ids, mgr)
		if err != nil {
			t.Fatal(err)
		}
		got = marksFromPlans(plans, "", func(string) bool { return true }, true)
		skip := map[catalog.ID]bool{}
		for _, id := range unmarked[mgr] {
			skip[id] = true
		}
		if len(got) != wantN[mgr] {
			t.Fatalf("%s marked %d %v", mgr, len(got), got)
		}
		for _, id := range ids {
			if skip[id] == got[id] {
				t.Fatalf("%s %s marked=%v", mgr, id, got[id])
			}
		}
		got = marksFromPlans(plans, "", func(string) bool { return false }, true)
		if mgr == "brew" {
			if len(got) != 1 || !got["compiler"] {
				t.Fatalf("%s xcode %v", mgr, got)
			}
		} else if len(got) != 0 {
			t.Fatalf("%s miss %v", mgr, got)
		}
	}
	got = marksFromPlans(brew, "", func(string) bool { return true }, false)
	if got["compiler"] || got["trash"] || got["mitos"] || len(got) != 32 {
		t.Fatalf("brew no xcode %v", got)
	}
	dnf, err := install.ResolveAll(ids, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	got = marksFromPlans(dnf, "", func(string) bool { return true }, false)
	if !got["compiler"] || len(got) != 32 {
		t.Fatalf("dnf ignores xcode %v", got)
	}
}

func TestDetectDoneAndRefresh(t *testing.T) {
	m := New(fedoraInfo())
	m.phase = phaseDetecting
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"whois": true},
	})
	mm := next.(model)
	if mm.phase != phaseHome || !mm.installed["whois"] || mm.scroll != 0 {
		t.Fatalf("phase %d %+v scroll %d", mm.phase, mm.installed, mm.scroll)
	}
	next, _ = m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Err:       errors.New("no pm"),
		Installed: map[catalog.ID]bool{"whois": true},
	})
	mm = next.(model)
	if mm.phase != phaseFailed || mm.installed != nil || mm.exitCode != 2 {
		t.Fatalf("failed %+v %d", mm.installed, mm.exitCode)
	}

	m = New(fedoraInfo())
	m.phase = phaseInstalling
	m.runID = 7
	m.events = make(chan bridgeMsg)
	next, cmd := m.onBridge(bridgeMsg{runID: 7, kind: kindDone, code: 0})
	mm = next.(model)
	if mm.phase != phaseReport || cmd == nil {
		t.Fatal("kindDone")
	}
	msg := cmd()
	im, ok := msg.(installedMsg)
	if !ok || im.runID != 7 {
		t.Fatalf("%T %+v", msg, msg)
	}
	m.runID = 8
	m.installed = map[catalog.ID]bool{"whois": true}
	next, cmd = m.Update(im)
	mm = next.(model)
	if cmd != nil || !mm.installed["whois"] || mm.installed["make"] {
		t.Fatal("stale refresh")
	}
	im.runID = 8
	im.installed = map[catalog.ID]bool{"make": true}
	next, _ = mm.Update(im)
	mm = next.(model)
	if !mm.installed["make"] || mm.installed["whois"] {
		t.Fatalf("%v", mm.installed)
	}
}

func TestSetDepsForce(t *testing.T) {
	m := SetDeps(New(fedoraInfo()), Deps{Force: true}).(model)
	if !m.force || m.deps.Force != true {
		t.Fatal("force not copied")
	}
}

type recordExec struct {
	mu    sync.Mutex
	argvs [][]string
	code  int
}

func (r *recordExec) Run(_ context.Context, argv []string) (string, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.argvs = append(r.argvs, append([]string(nil), argv...))
	return "", r.code, nil
}

func (r *recordExec) calls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]string(nil), r.argvs...)
}

// drain feeds the run's bridge messages through Update until the report.
func drain(t *testing.T, m model, cmd tea.Cmd) (model, []bridgeMsg) {
	t.Helper()
	var seen []bridgeMsg
	for n := 0; n < 200; n++ {
		if cmd == nil {
			t.Fatal("run stopped listening")
		}
		msg, ok := cmd().(bridgeMsg)
		if !ok {
			t.Fatalf("not bridge")
		}
		seen = append(seen, msg)
		next, c := m.Update(msg)
		m = next.(model)
		if msg.kind == kindDone {
			return m, seen
		}
		cmd = c
	}
	t.Fatal("no kindDone")
	return m, seen
}

func TestStartInstallPassesForce(t *testing.T) {
	for _, force := range []bool{false, true} {
		m := New(fedoraInfo())
		m.force = force
		m.info.Manager = "brew"
		m.steps = []install.Step{{
			ItemID:     "whois",
			Title:      "install whois",
			Kind:       install.KindPackage,
			Method:     pm.MethodPackage,
			Manager:    "brew",
			Argv:       []string{"brew", "install", "whois"},
			ProbeFiles: []string{"/usr/bin/whois"},
		}}
		rec := &recordExec{}
		m.deps.Runner.Exec = rec
		m.deps.Detect = func(string) map[catalog.ID]bool { return nil }
		m.deps.Runner.Exists = func(path string) bool { return path == "/usr/bin/whois" }
		next, cmd := m.launch(sudoCached, nil)
		mm := next.(model)
		if cmd == nil || mm.phase != phaseInstalling {
			t.Fatal("start")
		}
		mm, _ = drain(t, mm, cmd)
		asks := rec.calls()
		if mm.phase != phaseReport {
			t.Fatalf("phase %d", mm.phase)
		}
		if !force {
			if len(asks) != 0 || len(mm.results) != 1 || mm.results[0].Status != "satisfied" {
				t.Fatalf("skip asks %v %+v", asks, mm.results)
			}
			continue
		}
		if len(asks) != 1 || asks[0][0] != "brew" || asks[0][len(asks[0])-1] != "whois" {
			t.Fatalf("asks %v", asks)
		}
		if len(mm.results) != 1 || mm.results[0].Status != "ok" || mm.results[0].Note != pm.KegOnlyGenericNote {
			t.Fatalf("%+v", mm.results)
		}
	}
}

func cursorOn(m model, id catalog.ID) model {
	for i, cur := range m.flat {
		if cur == id {
			m.cursor = i
			return m
		}
	}
	return m
}

func TestInstalledLessRowUnchecked(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"less": true},
	})
	mm := next.(model)
	if mm.selected["less"] || !mm.selected["git"] || mm.scroll != 0 {
		t.Fatalf("selected less %v git %v", mm.selected["less"], mm.selected["git"])
	}
	mm = enterCatalog(t, mm)
	line := lineWith(render(mm, false), "less", "(已安裝)")
	if line == "" || !strings.Contains(line, "* [ ] less") || !strings.Contains(line, "[ ]") || strings.Contains(line, "[x]") {
		t.Fatalf("row %q", line)
	}
	if strings.Contains(render(mm, false), "\x1b") {
		t.Fatal("plain ansi")
	}

	styled := renderStyled(mm, testRenderer())
	sline := lineWith(styled, "less", "(已安裝)")
	if sline == "" || strings.Contains(sline, "[x]") || !strings.Contains(sline, "[ ]") || !strings.Contains(sline, "(已安裝)") {
		t.Fatalf("styled %s", sline)
	}

	failed, _ := New(fedoraInfo()).Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Err:       errors.New("no pm"),
		Installed: map[catalog.ID]bool{"less": true},
	})
	fm := failed.(model)
	if !fm.selected["less"] || fm.installed != nil {
		t.Fatal("error cleared defaults")
	}
	nilMap, _ := New(fedoraInfo()).Update(detectDoneMsg{Info: fedoraInfo()})
	nm := nilMap.(model)
	if !nm.selected["less"] || nm.installed != nil {
		t.Fatal("nil probe cleared defaults")
	}
}

// TestSpaceOnInstalledPicksReinstall: no force-reinstall y/n question any
// more; space on an installed tool picks it and the run re-downloads it.
func TestSpaceOnInstalledPicksReinstall(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"less": true},
	})
	mm := enterCatalog(t, cursorOn(next.(model), "less"))
	if mm.reinstalling() {
		t.Fatal("reinstalling before pick")
	}
	picked, cmd := mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil {
		t.Fatal("space command")
	}
	sel := picked.(model)
	if !sel.selected["less"] || sel.phase != phaseBrowsing || !sel.reinstalling() {
		t.Fatalf("pick less %v phase %d", sel.selected["less"], sel.phase)
	}
	view := render(sel, false)
	if strings.Contains(view, "要強制重新下載嗎？") || strings.Contains(view, "請加上 --force") {
		t.Fatal(view)
	}
	if line := lineWith(view, "less", "(已安裝)"); line == "" || !strings.Contains(line, "[x]") {
		t.Fatalf("row %q", line)
	}
	styled := renderStyled(sel, testRenderer())
	if strings.Contains(styled, "48;2;") || len(frameLines(styled)) != sel.height {
		t.Fatal("styled frame")
	}
	back, _ := sel.Update(tea.KeyMsg{Type: tea.KeySpace})
	if back.(model).selected["less"] {
		t.Fatal("second space")
	}
}

func TestUninstalledSpaceStillToggles(t *testing.T) {
	m := cursorOn(New(fedoraInfo()), "zsh")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm := next.(model)
	if !mm.selected["zsh"] || strings.Contains(render(mm, false), "要強制重新下載嗎？") {
		t.Fatal("uninstalled space")
	}
	next, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	if mm.selected["zsh"] {
		t.Fatal("second space")
	}
}

func TestForceSpaceTogglesInstalled(t *testing.T) {
	m := SetDeps(New(fedoraInfo()), Deps{Force: true}).(model)
	m.width = 80
	m.height = 40
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"less": true},
	})
	mm := enterCatalog(t, cursorOn(next.(model), "less"))
	if mm.selected["less"] || !mm.force {
		t.Fatal("setup")
	}
	next, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	view := render(mm, false)
	if !mm.selected["less"] || strings.Contains(view, "要強制重新下載嗎？") || strings.Contains(view, "請加上 --force") {
		t.Fatal(view)
	}
	next, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	if mm.selected["less"] {
		t.Fatal("second toggle")
	}
}

func TestASkipsInstalled(t *testing.T) {
	m := New(fedoraInfo())
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"less": true},
	})
	mm := enterCatalog(t, next.(model))
	next, _ = mm.Update(keyRune('a'))
	mm = next.(model)
	if mm.selected["less"] {
		t.Fatal("a selected less or opened the box")
	}
	next, _ = mm.Update(keyRune('a'))
	mm = next.(model)
	if mm.selected["less"] || !mm.selected["git"] {
		t.Fatal("a selected less on the way back on")
	}
	next, _ = mm.Update(keyRune('A'))
	mm = next.(model)
	if mm.selected["less"] {
		t.Fatal("A selected less")
	}
}

func enterCatalog(t *testing.T, m model) model {
	t.Helper()
	if m.phase != phaseHome {
		t.Fatalf("phase %d", m.phase)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter command")
	}
	mm := next.(model)
	if mm.phase != phaseBrowsing || mm.scroll != 0 {
		t.Fatalf("phase %d scroll %d", mm.phase, mm.scroll)
	}
	return mm
}

func TestBrowsingKeysDoNotScroll(t *testing.T) {
	m := New(fedoraInfo())
	m.installed = map[catalog.ID]bool{"whois": true}
	m.force = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm := next.(model)
	if mm.scroll != 0 || mm.cursor != 1 || !mm.force {
		t.Fatalf("scroll %d cursor %d", mm.scroll, mm.cursor)
	}
	if !strings.Contains(render(mm, false), "(已安裝)") {
		t.Fatal("force hid the marker")
	}
}
