package tui

import (
	"net"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/platform"
)

// conhostWidth measures a line the way the classic Windows console with a
// CJK code page draws it: East Asian ambiguous characters take two cells.
func conhostWidth(line string) int {
	c := runewidth.NewCondition()
	c.EastAsianWidth = true
	return c.StringWidth(ansi.Strip(line))
}

func conhostModel(t *testing.T, styled bool) model {
	t.Helper()
	info := platform.Info{
		OS:      platform.ClassifyOS("windows", platform.OSRelease{PrettyName: "Windows"}, false),
		Arch:    platform.ArchInfo{Family: "x86_64", GOARCH: "amd64"},
		Manager: "scoop",
	}
	m := SetDeps(New(info), Deps{GOOS: "windows", Styled: styled, ASCII: true, ReserveLastColumn: true}).(model)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = next.(model)
	m.phase = phaseHome
	m.homeUser, m.homeHost, m.homeIP, m.homeDNS = "user", "DESKTOP-IMOL8V5", "192.168.1.10", "192.168.1.1"
	return m
}

func TestConhostFramesFitWithoutWrapping(t *testing.T) {
	for _, styled := range []bool{false, true} {
		m := conhostModel(t, styled)
		if m.width != 79 {
			t.Fatalf("layout width %d, want 79 (last column reserved)", m.width)
		}
		frames := map[string]model{}
		frames["home0"] = m
		down := press(t, m, tea.KeyMsg{Type: tea.KeyDown})
		if down.homeIndex != 1 {
			t.Fatalf("down did not move: %d", down.homeIndex)
		}
		frames["home1"] = down
		up := press(t, down, tea.KeyMsg{Type: tea.KeyUp})
		frames["home-up"] = up
		browse := m
		browse.phase = phaseBrowsing
		frames["browse"] = browse
		un := m
		un.applyInstalled(map[catalog.ID]bool{"git": true, "python": true})
		next, _ := un.enterUninstall()
		frames["uninstall"] = next.(model)
		for name, fm := range frames {
			view := fm.View()
			lines := strings.Split(view, "\n")
			if len(lines) > 30 {
				t.Fatalf("%s styled=%v: %d lines > height", name, styled, len(lines))
			}
			for i, ln := range lines {
				if w := conhostWidth(ln); w > 79 {
					t.Fatalf("%s styled=%v line %d is %d cells on conhost (> 79), it wraps:\n%q", name, styled, i, w, ansi.Strip(ln))
				}
			}
		}
	}
}

func TestHomeMenuRowsAreUniqueAfterMoves(t *testing.T) {
	m := conhostModel(t, false)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown})
	view := m.View()
	if strings.Count(view, itemDownload) != 1 || strings.Count(view, itemUninstall) != 1 || strings.Count(view, "> ") != 1 {
		t.Fatalf("menu rows duplicated or cursor lost\n%s", view)
	}
	if !strings.Contains(view, "> "+itemUninstall) {
		t.Fatalf("cursor not on uninstall\n%s", view)
	}
}

func TestASCIIFrameKeepsWidths(t *testing.T) {
	in := "\x1b[1m╭──╮\x1b[0m │ 系統 … ↑↓ ┃━┏┓ 已安裝 ö"
	out := ASCIIFrame(in)
	if out != "\x1b[1m+--+\x1b[0m | 系統 . ^v |-++ 已安裝 o" {
		t.Fatalf("%q", out)
	}
	if ansi.StringWidth(in) != ansi.StringWidth(out) || conhostWidth(out) != ansi.StringWidth(out) {
		t.Fatalf("width changed: %d %d %d", ansi.StringWidth(in), ansi.StringWidth(out), conhostWidth(out))
	}
	if ASCIIFrame("plain") != "plain" {
		t.Fatal("ascii input changed")
	}
}

func TestFullWidthKeptWithoutWindowsFlags(t *testing.T) {
	m := New(fedoraInfo())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	if next.(model).width != 80 {
		t.Fatal("linux layout width changed")
	}
}

func TestWindowsArchWithoutUname(t *testing.T) {
	info := platform.Info{OS: platform.OSInfo{GOOS: "windows", SystemText: "Windows [GOOS=windows]"}, Arch: platform.ArchInfo{Family: "x86_64", GOARCH: "amd64"}, Manager: "scoop"}
	if got := homeArch(info); got != "x86_64（GOARCH=amd64）" {
		t.Fatal(got)
	}
	if got := FormatHeader(info); strings.Contains(got, "uname") || !strings.Contains(got, "套件: scoop") {
		t.Fatal(got)
	}
}

func TestDNSList(t *testing.T) {
	ips := []net.IP{net.ParseIP("fec0:0:0:ffff::1"), net.ParseIP("8.8.8.8"), net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8"), net.ParseIP("0.0.0.0")}
	if got := dnsList(ips); got != "8.8.8.8, 1.1.1.1" {
		t.Fatal(got)
	}
	if dnsList(nil) != identityFallback {
		t.Fatal("empty")
	}
}
