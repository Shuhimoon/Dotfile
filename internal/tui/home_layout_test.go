package tui

import (
	"net"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Plain frame: one-line title, hostname header, five colored-label rows.
func TestHomeFrame(t *testing.T) {
	m := homeFixture()
	m.homeIP = "172.20.1.5"
	m.homeDNS = "10.255.255.254"
	view := render(m, false)
	if strings.Contains(view, "\x1b") {
		t.Fatal("plain ansi")
	}
	if !strings.Contains(view, "Tools-install") || strings.Contains(view, "本機") || strings.Contains(view, "電腦名稱") || strings.Contains(view, "#") {
		t.Fatal(view)
	}
	lines := frameLines(view)
	if len(lines) != 40 || lines[len(lines)-1] != homeHelp {
		t.Fatalf("lines %d footer %q", len(lines), lines[len(lines)-1])
	}
	want := []string{
		"│ dotfiles",
		"│ 使用者  shuhi",
		"│ 系統    Fedora Linux 45",
		"│ 架構    ",
		"│ IP      172.20.1.5",
		"│ DNS     10.255.255.254",
	}
	at := -1
	for i, ln := range lines {
		if strings.Contains(ln, want[0]) {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatal(view)
	}
	if !strings.Contains(lines[at+1], "├") {
		t.Fatalf("rule %q", lines[at+1])
	}
	for k, w := range want[1:] {
		ln := lines[at+2+k]
		if !strings.Contains(ln, w) || !strings.HasSuffix(strings.TrimRight(ln, " "), "│") {
			t.Fatalf("row %d %q want %q", k, ln, w)
		}
	}
	for _, ln := range lines {
		if ln != "" && ansi.StringWidth(ln) != 80 && ln != homeHelp {
			t.Fatalf("width %d %q", ansi.StringWidth(ln), ln)
		}
	}
}

// Styled frame: block-letter title, gradient label colors, title-colored selection.
func TestHomeStyled(t *testing.T) {
	m := homeFixture()
	view := renderStyled(m, testRenderer())
	lines := frameLines(view)
	if len(lines) != 40 {
		t.Fatalf("lines %d", len(lines))
	}
	if strings.Contains(ansi.Strip(view), "Tools-install") {
		t.Fatal("big title expected")
	}
	for _, bg := range []string{"1;38;2;240;173;160", "1;38;2;158;226;240"} {
		if !strings.Contains(view, bg) {
			t.Fatalf("title block %s", bg)
		}
	}
	rows := bigTitleRows()
	found := 0
	for _, ln := range lines {
		s := ansi.Strip(ln)
		if strings.ContainsAny(s, bigStrokes) {
			found++
		}
	}
	if found != len(rows) {
		t.Fatalf("big rows %d", found)
	}
	var head, user, dns, sel string
	for _, ln := range lines {
		s := ansi.Strip(ln)
		switch {
		case strings.Contains(s, "│ dotfiles"):
			head = ln
		case strings.Contains(s, "使用者"):
			user = ln
		case strings.Contains(s, "DNS"):
			dns = ln
		case strings.Contains(s, "> tools-download"):
			sel = ln
		}
	}
	if !strings.Contains(head, "1;38;2;247;198;186") {
		t.Fatalf("head %q", head)
	}
	if !strings.Contains(user, "1;38;2;240;173;160") || !strings.Contains(user, "38;2;237;227;225") {
		t.Fatalf("user %q", user)
	}
	if !strings.Contains(dns, "1;38;2;158;226;240") {
		t.Fatalf("dns %q", dns)
	}
	if !strings.Contains(sel, "48;2;240;173;160") || !strings.Contains(sel, "1;38;2;30;20;22") {
		t.Fatalf("select %q", sel)
	}
}

// Narrow or short terminals fall back to the one-line title.
func TestHomeNarrow(t *testing.T) {
	m := homeFixture()
	m.width = 40
	pv := render(m, false)
	if !strings.Contains(pv, "Tools-install") || !strings.Contains(pv, "…") || !strings.Contains(pv, "╭") || !strings.Contains(pv, "╰") {
		t.Fatal(pv)
	}
	sv := ansi.Strip(renderStyled(m, testRenderer()))
	if !strings.Contains(sv, "Tools-install") {
		t.Fatal(sv)
	}

	short := homeFixture()
	short.height = 12
	if !strings.Contains(ansi.Strip(renderStyled(short, testRenderer())), "Tools-install") {
		t.Fatal("short styled fallback")
	}
	short.height = 3
	sl := frameLines(render(short, false))
	if len(sl) != 3 || !strings.Contains(sl[0], "Tools-install") || !strings.HasPrefix(sl[1], "…還有") || sl[2] != homeHelp {
		t.Fatalf("clip %q", sl)
	}

	zero := homeFixture()
	zero.width = 0
	zero.height = 5
	if len(frameLines(render(zero, false))) != 5 || len(frameLines(renderStyled(zero, testRenderer()))) != 5 {
		t.Fatal("width 0")
	}

	tiny := homeFixture()
	tiny.width = 4
	tiny.height = 10
	tv := render(tiny, false)
	if strings.Contains(tv, "\x1b") || !strings.Contains(tv, "Too…") {
		t.Fatal(tv)
	}
}

func TestUnavailableFrame(t *testing.T) {
	m := homeFixture()
	m.phase = phaseUnavailable
	view := render(m, false)
	if !strings.Contains(view, "tools-uninstall") || !strings.Contains(view, unavailSentence) || !strings.Contains(view, "Tools-install") {
		t.Fatal(view)
	}
	lines := frameLines(view)
	if lines[len(lines)-1] != unavailableHelp {
		t.Fatalf("footer %q", lines[len(lines)-1])
	}
	for _, bad := range []string{"apt-get", "dnf", "yum", "pacman", "brew uninstall"} {
		if strings.Contains(view, bad) {
			t.Fatal(bad)
		}
	}
	styled := renderStyled(m, testRenderer())
	if !strings.Contains(styled, "38;2;240;173;160") || !strings.Contains(styled, "1;38;2;247;198;186") || !strings.Contains(styled, "38;2;237;227;225") {
		t.Fatal(styled)
	}
	narrow := m
	narrow.width = 4
	narrow.height = 10
	nv := render(narrow, false)
	if strings.Contains(nv, "\x1b") || !strings.Contains(nv, "Too…") || !strings.Contains(nv, "不…") {
		t.Fatal(nv)
	}
}

func TestPickIP(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.3.4"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("192.168.1.5"), Mask: net.CIDRMask(24, 32)},
	}
	if got := pickIP(addrs); got != "192.168.1.5" {
		t.Fatal(got)
	}
	if got := pickIP(nil); got != identityFallback {
		t.Fatal(got)
	}
}

func TestParseResolv(t *testing.T) {
	got := parseResolv("# wsl\nnameserver 10.255.255.254\nsearch lan\nnameserver 8.8.8.8\n")
	if got != "10.255.255.254, 8.8.8.8" {
		t.Fatal(got)
	}
	if got := parseResolv("search lan\n"); got != identityFallback {
		t.Fatal(got)
	}
}

func TestIdentityNetwork(t *testing.T) {
	m := homeFixture()
	next, _ := m.Update(identityMsg{User: "shuhi", Host: "box", IP: "1.2.3.4", DNS: "8.8.8.8"})
	mm := next.(model)
	if mm.homeIP != "1.2.3.4" || mm.homeDNS != "8.8.8.8" || mm.homeHost != "box" {
		t.Fatalf("%+v %+v", mm.homeIP, mm.homeDNS)
	}
}
