package tui

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/platform"
)

func TestResolveIdentity(t *testing.T) {
	ok := func(s string) func() (string, error) {
		return func() (string, error) { return s, nil }
	}
	userName, host := resolveIdentity(ok("shuhi"), ok("dotfiles"))
	if userName != "shuhi" || host != "dotfiles" {
		t.Fatalf("got %q %q", userName, host)
	}
	bad := func() (string, error) { return "secret", errors.New("no") }
	cases := []func() (string, error){
		bad,
		func() (string, error) { return "", nil },
		func() (string, error) { return "   ", nil },
		func() (string, error) { return "shuhi\n", nil },
		func() (string, error) { return "a\x1b[31m", nil },
		func() (string, error) { return "a\x7f", nil },
		nil,
	}
	for i, fn := range cases {
		u, h := resolveIdentity(fn, fn)
		if u != "未知" || h != "未知" {
			t.Fatalf("case %d %q %q", i, u, h)
		}
	}

	m := New(fedoraInfo())
	m.phase = phaseBrowsing
	m.installed = map[catalog.ID]bool{"git": true}
	next, cmd := m.Update(identityMsg{User: "shuhi", Host: "dotfiles"})
	if cmd != nil {
		t.Fatal("identity command")
	}
	mm := next.(model)
	if mm.phase != phaseBrowsing || mm.homeUser != "shuhi" || mm.homeHost != "dotfiles" || mm.scroll != 0 || !mm.installed["git"] {
		t.Fatalf("identity changed model %+v", mm.homeUser)
	}
}

func homeFixture() model {
	m := New(fedoraInfo())
	m.phase = phaseHome
	m.homeUser = "shuhi"
	m.homeHost = "dotfiles"
	m.width = 80
	m.height = 40
	return m
}

func TestDetectOpensHome(t *testing.T) {
	m := New(platform.Info{})
	m.width = 80
	m.height = 40
	next, cmd := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Installed: map[catalog.ID]bool{"whois": true},
	})
	if cmd != nil {
		t.Fatal("cmd")
	}
	mm := next.(model)
	if mm.phase != phaseHome || !mm.installed["whois"] || mm.scroll != 0 {
		t.Fatalf("phase %d %+v", mm.phase, mm.installed)
	}
	view := render(mm, false)
	if !strings.Contains(view, "Tools-install") || strings.Contains(view, "#") || strings.Contains(view, "任意鍵關閉") || strings.Contains(view, "\x1b") {
		t.Fatal(view)
	}

	bad, _ := m.Update(detectDoneMsg{
		Info:      fedoraInfo(),
		Err:       errors.New("no pm"),
		Installed: map[catalog.ID]bool{"whois": true},
	})
	bm := bad.(model)
	if bm.phase != phaseFailed || bm.installed != nil || bm.exitCode != 2 {
		t.Fatalf("failed %+v %d", bm.installed, bm.exitCode)
	}
	failed := render(bm, false)
	if strings.Contains(failed, "Tools-install") || strings.Contains(failed, "#") {
		t.Fatal("failed drew home")
	}

	stayed := New(fedoraInfo())
	stayed.phase = phaseReport
	stayed.runID = 3
	next, _ = stayed.Update(installedMsg{runID: 3, installed: map[catalog.ID]bool{"git": true}})
	if next.(model).phase != phaseReport {
		t.Fatal("refresh returned home")
	}
}

func cellsWithBG(s, bg string) int {
	cur := ""
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := strings.IndexByte(s[i:], 'm')
			if j < 0 {
				break
			}
			params := s[i+2 : i+j]
			switch {
			case params == "" || params == "0":
				cur = ""
			case strings.Contains(params, bg):
				cur = bg
			case strings.Contains(params, "48;2;"):
				cur = "other"
			}
			i += j + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if cur == bg {
			n += ansi.StringWidth(string(r))
		}
		i += size
	}
	return n
}
