package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
)

func TestMarksFromPlans(t *testing.T) {
	ids := catalog.IDs()
	if len(ids) != 32 {
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
		"apt":    {"zellij", "mitos"},
		"dnf":    {"starship", "zellij", "mitos"},
		"yum":    {"starship", "eza", "fastfetch", "zellij", "mitos"},
		"pacman": {"mitos"},
		"zypper": {"mitos"},
		"apk":    {"mitos"},
		"xbps":   {"mitos"},
		"brew":   {"trash", "mitos"},
	}
	wantN := map[string]int{
		"apt": 30, "dnf": 29, "yum": 27,
		"pacman": 31, "zypper": 31, "apk": 31, "xbps": 31,
		"brew": 30,
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
	if got["compiler"] || got["trash"] || got["mitos"] || len(got) != 29 {
		t.Fatalf("brew no xcode %v", got)
	}
	dnf, err := install.ResolveAll(ids, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	got = marksFromPlans(dnf, "", func(string) bool { return true }, false)
	if !got["compiler"] || len(got) != 29 {
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

func TestStartInstallPassesForce(t *testing.T) {
	for _, force := range []bool{false, true} {
		m := New(fedoraInfo())
		m.force = force
		m.phase = phaseConfirm
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
		m.deps.Runner.Exists = func(path string) bool { return path == "/usr/bin/whois" }
		next, cmd := m.startInstall()
		mm := next.(model)
		t.Cleanup(func() {
			if mm.cancel != nil {
				mm.cancel()
			}
		})
		if cmd == nil || mm.scroll != 0 {
			t.Fatal("start")
		}
		var asks [][]string
		var done bridgeMsg
		for n := 0; n < 8; n++ {
			msg, ok := cmd().(bridgeMsg)
			if !ok {
				t.Fatal("not bridge")
			}
			switch msg.kind {
			case kindAsk:
				asks = append(asks, append([]string(nil), msg.argv...))
				msg.reply <- execReply{}
			case kindDone:
				done = msg
			}
			if msg.kind == kindDone {
				break
			}
			cmd = listen(mm.events, mm.runID)
		}
		if done.kind != kindDone {
			t.Fatalf("force %v no done asks %v", force, asks)
		}
		if !force {
			if len(asks) != 0 || len(done.results) != 1 || done.results[0].Status != "satisfied" {
				t.Fatalf("skip asks %v %+v", asks, done.results)
			}
			continue
		}
		if len(asks) != 1 || asks[0][0] != "brew" || asks[0][len(asks[0])-1] != "whois" {
			t.Fatalf("asks %v", asks)
		}
		if len(done.results) != 1 || done.results[0].Status != "ok" || done.results[0].Note != pm.KegOnlyGenericNote {
			t.Fatalf("%+v", done.results)
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

func TestReinstallAsk(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	next, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"less": true},
	})
	mm := enterCatalog(t, cursorOn(next.(model), "less"))
	opened, cmd := mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil {
		t.Fatal("space command")
	}
	ask := opened.(model)
	if ask.selected["less"] || ask.force || ask.askName != "less" || ask.askTold || ask.scroll != 0 {
		t.Fatalf("ask %+v selected %v", ask.askName, ask.selected["less"])
	}
	view := render(ask, false)
	if strings.Contains(view, "\x1b") || !strings.Contains(view, "less 已安裝。要強制重新下載嗎？") {
		t.Fatal(view)
	}
	lines := frameLines(view)
	if len(lines) != ask.height || lines[len(lines)-1] != browsingHelp(ask.keys) {
		t.Fatalf("frame %d %q", len(lines), lines[len(lines)-1])
	}
	if line := lineWith(view, "less", "(已安裝)"); line == "" || strings.Contains(line, "[x]") {
		t.Fatalf("row hidden %q", line)
	}

	styled := renderStyled(ask, testRenderer())
	slines := frameLines(styled)
	if len(slines) != ask.height {
		t.Fatalf("styled lines %d", len(slines))
	}
	qline := lineContaining(styled, "less 已安裝。要強制重新下載嗎？")
	if qline == "" || !strings.Contains(qline, sgrHeader) || !strings.Contains(qline, sgrCream) || strings.Contains(qline, sgrTitle) || ansi.StringWidth(qline) != ask.width {
		t.Fatalf("box %s", qline)
	}
	if !strings.Contains(slines[len(slines)-1], sgrFooter) {
		t.Fatal("footer left the last line")
	}

	told, cmd := ask.Update(keyRune('y'))
	if cmd != nil {
		t.Fatal("y command")
	}
	ym := told.(model)
	yview := render(ym, false)
	if ym.selected["less"] || ym.force || ym.deps.Runner.Force || ym.phase != phaseBrowsing {
		t.Fatal("y changed install state")
	}
	if !strings.Contains(yview, "請加上 --force 後重新執行") || strings.Contains(yview, "要強制重新下載嗎？") {
		t.Fatal(yview)
	}
	if strings.Contains(yview, "\x1b") {
		t.Fatal("plain y ansi")
	}

	for _, k := range []tea.KeyMsg{keyRune('n'), tea.KeyMsg{Type: tea.KeyEsc}, keyRune('q')} {
		closed, cmd := ask.Update(k)
		if cmd != nil {
			t.Fatalf("%s command", k.String())
		}
		cm := closed.(model)
		if cm.selected["less"] || cm.askName != "" || strings.Contains(render(cm, false), "要強制重新下載嗎？") {
			t.Fatalf("%s left the box", k.String())
		}
	}

	snap := copySel(ask.selected)
	for _, k := range []tea.KeyMsg{
		keyRune('j'), keyRune('k'), tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyUp},
		tea.KeyMsg{Type: tea.KeySpace}, tea.KeyMsg{Type: tea.KeyEnter},
		keyRune('a'), keyRune('A'), keyRune('d'), keyRune('g'), keyRune('G'),
	} {
		stayed, cmd := ask.Update(k)
		sm := stayed.(model)
		if cmd != nil || sm.cursor != ask.cursor || sm.phase != phaseBrowsing || sm.askName != "less" || sm.askTold || sm.scroll != 0 || !sameSel(sm.selected, snap) {
			t.Fatalf("%s changed the catalog", k.String())
		}
	}

	narrow := ask
	narrow.width = 12
	narrow.height = 8
	nlines := frameLines(render(narrow, false))
	if len(nlines) != narrow.height || nlines[len(nlines)-1] != browsingHelp(narrow.keys) {
		t.Fatalf("narrow %d %q", len(nlines), nlines)
	}
	if strings.Contains(strings.Join(nlines, "\n"), "less 已安裝。要強制重新下載嗎？") || strings.Contains(nlines[len(nlines)-1], "要強制重新下載") {
		t.Fatal("question wrapped into the footer")
	}
}

func TestUninstalledSpaceStillToggles(t *testing.T) {
	m := cursorOn(New(fedoraInfo()), "zsh")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm := next.(model)
	if !mm.selected["zsh"] || mm.askName != "" || strings.Contains(render(mm, false), "要強制重新下載嗎？") {
		t.Fatal("uninstalled space")
	}
	next, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	if mm.selected["zsh"] || mm.askName != "" {
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
	if !mm.selected["less"] || mm.askName != "" || strings.Contains(view, "要強制重新下載嗎？") || strings.Contains(view, "請加上 --force") {
		t.Fatal(view)
	}
	next, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = next.(model)
	if mm.selected["less"] || mm.askName != "" {
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
	if mm.selected["less"] || mm.askName != "" {
		t.Fatal("a selected less or opened the box")
	}
	next, _ = mm.Update(keyRune('a'))
	mm = next.(model)
	if mm.selected["less"] || mm.askName != "" || !mm.selected["git"] {
		t.Fatal("a selected less on the way back on")
	}
	next, _ = mm.Update(keyRune('A'))
	mm = next.(model)
	if mm.selected["less"] || mm.askName != "" {
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
