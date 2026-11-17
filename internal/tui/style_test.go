package tui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

const (
	sgrClay      = "38;2;240;173;160"
	sgrSand      = "38;2;217;207;205"
	sgrGold      = "38;2;247;198;186"
	sgrOlive     = "38;2;240;173;160" // first category header: start of the title gradient
	sgrCream     = "38;2;237;227;225"
	sgrKey       = "38;2;158;226;240"
	sgrMuted     = "38;2;156;143;146"
	sgrBark      = "48;2;30;26;29"
	sgrHeader    = "48;2;58;44;48"
	sgrInstalled = "48;2;29;46;51"
	sgrTitle     = "48;2;43;34;38"
	sgrCursor    = "48;2;240;173;160"
	sgrFooter    = "48;2;43;34;38"
	sgrMode      = "48;2;30;26;29"
	sgrInk       = "38;2;30;20;22"
)

func testRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	r.SetHasDarkBackground(true)
	return r
}

func lineContaining(view, needle string) string {
	for _, ln := range frameLines(view) {
		if strings.Contains(ansi.Strip(ln), needle) {
			return ln
		}
	}
	return ""
}

func lineWith(view, a, b string) string {
	for _, ln := range frameLines(view) {
		if strings.Contains(ln, a) && strings.Contains(ln, b) {
			return ln
		}
	}
	return ""
}

func TestStyledCheckedAndUncheckedColors(t *testing.T) {
	m := New(fedoraInfo())
	view := renderStyled(m, testRenderer())
	curl := lineWith(view, "[x]", "curl")
	zsh := lineWith(view, "[ ]", "zsh")
	if curl == "" || zsh == "" {
		t.Fatal("missing rows")
	}
	if !strings.Contains(curl, sgrClay) || !strings.Contains(curl, sgrGold) || strings.Contains(curl, sgrSand) {
		t.Fatalf("curl %s", curl)
	}
	if !strings.Contains(zsh, sgrSand) || strings.Contains(zsh, sgrClay) || strings.Contains(zsh, sgrGold) {
		t.Fatalf("zsh %s", zsh)
	}
}

func TestStyledCursorOverlayDistinct(t *testing.T) {
	m := New(fedoraInfo())
	view := renderStyled(m, testRenderer())
	git := lineWith(view, "[x]", "git")
	curl := lineWith(view, "[x]", "curl")
	if git == "" || curl == "" {
		t.Fatal("missing rows")
	}
	if !strings.Contains(git, sgrCursor) || !strings.Contains(git, sgrInk) || strings.Contains(git, sgrBark) {
		t.Fatalf("git %s", git)
	}
	if strings.Contains(curl, sgrCursor) {
		t.Fatalf("curl has cursor bg %s", curl)
	}
}

func TestStyledUncheckedCursorKeepsSand(t *testing.T) {
	m := New(fedoraInfo())
	for i, id := range m.flat {
		if id == "zsh" {
			m.cursor = i
		}
	}
	view := renderStyled(m, testRenderer())
	zsh := lineWith(view, "[ ]", "zsh")
	if zsh == "" {
		t.Fatal("missing zsh")
	}
	if !strings.Contains(zsh, sgrCursor) || !strings.Contains(zsh, sgrInk) || strings.Contains(zsh, sgrSand) {
		t.Fatalf("zsh %s", zsh)
	}
}

func TestStyledStarIsNotCheckColor(t *testing.T) {
	m := New(fedoraInfo())
	for i, id := range m.flat {
		if id == "zsh" {
			m.cursor = i // keep the cursor (ink on peach) off the starred rows
		}
	}
	view := renderStyled(m, testRenderer())
	frag := styledFragment(view, "*")
	if frag == "" || !strings.Contains(frag, "*") {
		t.Fatal("missing star fragment")
	}
	if !strings.Contains(frag, sgrGold) || strings.Contains(frag, sgrClay) || strings.Contains(frag, sgrKey) {
		t.Fatalf("star %q", frag)
	}
}

func TestStyledTitleBarUsesRaisedDark(t *testing.T) {
	m := New(fedoraInfo())
	view := renderStyled(m, testRenderer())
	line := lineContaining(view, "系統:")
	// The title bar text runs along the home gradient, starting at #F0ADA0.
	want := "\x1b[1;38;2;240;173;160;" + sgrTitle + "m"
	if line == "" || !strings.Contains(line, want) || strings.Contains(line, sgrBark) || strings.Contains(line, sgrHeader) || strings.Contains(line, sgrInstalled) {
		t.Fatalf("header %s", line)
	}
	if ansi.StringWidth(line) != m.width {
		t.Fatalf("header width %d", ansi.StringWidth(line))
	}
}

func TestStyledCategoryHeaderOlive(t *testing.T) {
	m := New(fedoraInfo())
	view := renderStyled(m, testRenderer())
	line := lineContaining(view, "基礎")
	if line == "" || !strings.Contains(line, sgrOlive) || !strings.Contains(line, sgrBark) {
		t.Fatalf("header %s", line)
	}
	if ansi.StringWidth(line) != m.width {
		t.Fatalf("category width %d", ansi.StringWidth(line))
	}
}

func TestDescriptionUsesMutedNotCheckColor(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 100
	view := renderStyled(m, testRenderer())
	line := lineContaining(view, "Fedora 套件是 wget2-wget")
	if line == "" {
		t.Fatal("missing description")
	}
	if !strings.Contains(line, sgrMuted) || !strings.Contains(line, sgrClay) {
		t.Fatalf("wget %s", line)
	}
	sep := "\x1b[" + sgrMuted + ";" + sgrBark + "m  \x1b[0m"
	if !strings.Contains(line, sep) || hasReverse(styledFragment(line, "Fedora")) {
		t.Fatalf("uninstalled sep %s", line)
	}
}

func TestStyledStatusAndConfirmBody(t *testing.T) {
	m := New(fedoraInfo())
	view := renderStyled(m, testRenderer())
	status := lineContaining(view, "游標")
	if status == "" || !strings.Contains(status, sgrMuted) || !strings.Contains(status, sgrBark) {
		t.Fatalf("status %s", status)
	}

	m.phase = phaseConfirm
	m.steps = []install.Step{{Title: "計畫安裝 curl"}}
	view = renderStyled(m, testRenderer())
	step := lineContaining(view, "計畫安裝 curl")
	if step == "" || !strings.Contains(step, sgrCream) || !strings.Contains(step, sgrBark) {
		t.Fatalf("step %s", step)
	}
	filled := 0
	for _, ln := range frameLines(view) {
		if strings.TrimSpace(ansi.Strip(ln)) == "" && strings.Contains(ln, sgrBark) {
			filled++
		}
	}
	if filled == 0 {
		t.Fatal("short body was not filled with bark")
	}

	m.phase = phaseReport
	m.steps = nil
	m.results = []install.ItemResult{{ID: "git", Status: "ok", Note: "note"}}
	view = renderStyled(m, testRenderer())
	rep := lineContaining(view, "git  ok  note")
	if rep == "" || !strings.Contains(rep, sgrCream) || !strings.Contains(rep, sgrBark) {
		t.Fatalf("report %s", rep)
	}
	if strings.Contains(rep, sgrClay) || strings.Contains(rep, sgrGold) {
		t.Fatalf("report recolored %s", rep)
	}

	m.phase = phaseInstalling
	m.results = nil
	m.curTitle = "rust"
	m.done = []doneLine{
		{Title: "套件甲", Status: "ok"},
		{Title: "套件乙", Status: "failed"},
		{Title: "套件丙", Status: "satisfied"},
	}
	m.steps = []install.Step{{Title: "套件丁"}}
	view = renderStyled(m, testRenderer())
	for _, needle := range []string{"  套件甲  ok", "  套件乙  failed", "  套件丙  satisfied"} {
		line := lineContaining(view, needle)
		if line == "" || !strings.Contains(line, sgrCream) || !strings.Contains(line, sgrBark) {
			t.Fatalf("%s %s", needle, line)
		}
		if strings.Contains(line, sgrClay) || strings.Contains(line, sgrGold) || strings.Contains(line, sgrOlive) {
			t.Fatalf("%s recolored %s", needle, line)
		}
		if strings.Contains(line, "\x1b[7;") || strings.Contains(line, "\x1b[1;7;") || strings.Contains(line, sgrInstalled) || strings.Contains(line, sgrTitle) {
			t.Fatalf("%s reversed %s", needle, line)
		}
	}
	if strings.Contains(rep, "\x1b[7;") || strings.Contains(rep, "\x1b[1;7;") || strings.Contains(rep, sgrInstalled) || strings.Contains(rep, sgrTitle) {
		t.Fatalf("report reversed %s", rep)
	}
	if strings.Contains(status, sgrInstalled) || strings.Contains(status, sgrTitle) || strings.Contains(step, sgrInstalled) || strings.Contains(step, sgrTitle) {
		t.Fatal("status or confirm picked up the row or title background")
	}
}

func TestInstalledRowPinsMarkerOnPurple(t *testing.T) {
	r := testRenderer()
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	m.installed = map[catalog.ID]bool{"whois": true}
	// cursor stays on git: the peach cursor row is checked elsewhere.
	view := renderStyled(m, r)
	whois := lineWith(view, "whois", "(已安裝)")
	if whois == "" {
		t.Fatal("missing whois")
	}
	if ansi.StringWidth(whois) != 80 {
		t.Fatalf("whois width %d", ansi.StringWidth(whois))
	}
	if !strings.HasPrefix(whois, "  ") || strings.Contains(whois[:2], "\x1b") {
		t.Fatal("left inset")
	}
	if !strings.HasSuffix(whois, "  ") || strings.Contains(whois[len(whois)-2:], "\x1b") {
		t.Fatal("right inset")
	}
	if hasReverse(whois) || strings.Contains(whois, sgrCursor) || strings.Contains(whois, sgrBark) || strings.Contains(whois, sgrMuted) {
		t.Fatalf("whois %s", whois)
	}
	star := "\x1b[" + sgrGold + ";" + sgrInstalled + "m*\x1b[0m"
	box := "\x1b[" + sgrClay + ";" + sgrInstalled + "m[x]\x1b[0m"
	mark := "\x1b[" + sgrKey + ";" + sgrInstalled + "m(已安裝)\x1b[0m"
	if !strings.Contains(whois, star) || !strings.Contains(whois, box) || !strings.Contains(whois, mark) {
		t.Fatalf("whois %s", whois)
	}
	starFrag := styledFragment(whois, "*")
	if !strings.Contains(starFrag, sgrGold) || strings.Contains(starFrag, sgrClay) || hasReverse(starFrag) {
		t.Fatalf("star %q", starFrag)
	}
	if !strings.Contains(whois, "\x1b["+sgrInstalled+"m"+strings.Repeat(" ", 50)+"\x1b[0m") {
		t.Fatal("missing purple gap")
	}

	zshLine := lineWith(view, "[ ]", "zsh")
	if zshLine == "" || hasReverse(zshLine) || strings.Contains(zshLine, "(已安裝)") || strings.Contains(zshLine, sgrInstalled) {
		t.Fatalf("zsh %s", zshLine)
	}
	if ansi.StringWidth(zshLine) != 80 || strings.Contains(zshLine, sgrGold) || strings.Contains(zshLine, sgrClay) {
		t.Fatalf("zsh style %s", zshLine)
	}

	m.cursor = 0
	m.installed["zsh"] = true
	m.selected["zsh"] = false
	view = renderStyled(m, r)
	marked := lineWith(view, "zsh", "(已安裝)")
	if marked == "" || strings.Contains(marked, "*") || strings.Contains(marked, "[x]") || strings.Contains(marked, sgrGold) || strings.Contains(marked, sgrClay) {
		t.Fatalf("independent %s", marked)
	}
	if hasReverse(marked) || !strings.Contains(marked, "\x1b["+sgrKey+";"+sgrInstalled+"m(已安裝)\x1b[0m") {
		t.Fatalf("sand marker %s", marked)
	}

	m.width = 100
	m.cursor = 0
	m.installed = map[catalog.ID]bool{"wget": true}
	view = renderStyled(m, r)
	wget := lineContaining(view, "Fedora 套件是 wget2-wget")
	if wget == "" || ansi.StringWidth(wget) != 100 || hasReverse(wget) {
		t.Fatalf("wget %d %s", ansi.StringWidth(wget), wget)
	}
	desc := "\x1b[" + sgrSand + ";" + sgrInstalled + "mFedora 套件是 wget2-wget\x1b[0m"
	gap := "\x1b[" + sgrInstalled + "m" + strings.Repeat(" ", 44) + "\x1b[0m"
	if !strings.Contains(wget, desc) || !strings.Contains(wget, gap) || strings.Contains(wget, sgrMuted) || strings.Contains(wget, sgrBark) || strings.Contains(wget, sgrCursor) {
		t.Fatalf("desc %s", wget)
	}
	mark = styledFragment(wget, "(已安裝)")
	if !strings.Contains(mark, sgrKey+";"+sgrInstalled) || hasReverse(mark) {
		t.Fatalf("mark %q", mark)
	}

	for i, id := range m.flat {
		if id == "wget" {
			m.cursor = i
		}
	}
	view = renderStyled(m, r)
	wget = lineContaining(view, "Fedora 套件是 wget2-wget")
	// The cursor row is the home menu highlight even on an installed row: peach fill, ink text.
	cur := "\x1b[1;" + sgrInk + ";" + sgrCursor + "m>\x1b[0m"
	inkMark := "\x1b[" + sgrInk + ";" + sgrCursor + "m(已安裝)\x1b[0m"
	if !strings.Contains(wget, cur) || !strings.Contains(wget, inkMark) || strings.Contains(wget, sgrInstalled) || strings.Contains(wget, sgrMuted) || hasReverse(wget) {
		t.Fatalf("cursor wget %s", wget)
	}

	for _, ln := range frameLines(renderStyled(New(fedoraInfo()), r)) {
		if ansi.StringWidth(ln) != 80 || strings.Contains(ln, sgrInstalled) {
			t.Fatalf("row width %d %s", ansi.StringWidth(ln), ln)
		}
	}

	narrow := New(fedoraInfo())
	narrow.width = 26
	narrow.installed = map[catalog.ID]bool{"git": true}
	narrow.cursor = 1 // off git, so the installed style is visible
	nline := lineWith(renderStyled(narrow, r), "[x]", "git")
	if nline == "" || ansi.StringWidth(nline) != 26 || !strings.Contains(nline, "[x]") || !strings.Contains(nline, "git") || !strings.Contains(nline, sgrInstalled) || hasReverse(nline) || !strings.Contains(nline, "(已安裝)") {
		t.Fatalf("narrow %d %s", ansi.StringWidth(nline), nline)
	}

	narrow.width = 4
	nline = lineWith(renderStyled(narrow, r), "(已", "")
	if nline != "" && (!strings.Contains(ansi.Strip(nline), "(已…") || !strings.Contains(nline, sgrKey+";"+sgrInstalled) || hasReverse(nline)) {
		t.Fatalf("width 4 %s", nline)
	}
	four := lineContaining(renderStyled(narrow, r), "(已")
	if ansi.StringWidth(four) != 4 || !strings.Contains(ansi.Strip(four), "(已…") {
		t.Fatalf("width 4 line %d %s", ansi.StringWidth(four), four)
	}
}

func hasReverse(s string) bool {
	return strings.Contains(s, "\x1b[7;") || strings.Contains(s, "\x1b[1;7;")
}

func TestNoColorIsPlainText(t *testing.T) {
	m := New(fedoraInfo())
	view := render(m, false)
	if strings.Contains(view, "\x1b") {
		t.Fatal("plain render emitted ANSI")
	}
	if !strings.Contains(view, "> * [x] git") || !strings.Contains(view, "    [ ] zsh") {
		t.Fatalf("%s", view)
	}
	home := New(fedoraInfo())
	home.phase = phaseHome
	home.homeUser = "shuhi"
	home.homeHost = "dotfiles"
	hv := render(home, false)
	if strings.Contains(hv, "\x1b") {
		t.Fatal("home plain ansi")
	}
	if !strings.Contains(hv, "Tools-install") || strings.Contains(hv, "#") {
		t.Fatal(hv)
	}
}

func TestStyledFooterBackground(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	lines := frameLines(renderStyled(m, testRenderer()))
	if len(lines) < 2 {
		t.Fatal("short frame")
	}
	foot := lines[len(lines)-2:]
	for _, ln := range foot {
		if !strings.Contains(ln, sgrFooter) {
			t.Fatalf("footer line missing bg %s", ln)
		}
		if ansi.StringWidth(ln) != 80 {
			t.Fatalf("footer width %d", ansi.StringWidth(ln))
		}
	}
	last := foot[1]
	q := strings.Index(last, "q")
	leave := strings.Index(last, "離開")
	if q < 0 || leave < 0 || q >= leave {
		t.Fatalf("q/leave %q", last)
	}
	between := last[q:leave]
	if !strings.Contains(between, sgrFooter) {
		t.Fatalf("gap %q", between)
	}
	if !strings.Contains(sgrBefore(last, q), sgrKey) {
		t.Fatalf("q color %q", sgrBefore(last, q))
	}
	if !strings.Contains(sgrBefore(last, leave), sgrCream) {
		t.Fatalf("leave color %q", sgrBefore(last, leave))
	}
	mode := sgrBefore(foot[0], strings.Index(foot[0], "操作"))
	if !strings.Contains(mode, sgrMode) {
		t.Fatalf("mode %q", mode)
	}
}

func styledFragment(view, token string) string {
	i := strings.Index(view, token)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(view[:i], "\x1b[")
	if start < 0 {
		start = 0
	}
	return view[start : i+len(token)]
}

func sgrBefore(s string, idx int) string {
	if idx < 0 {
		return ""
	}
	start := strings.LastIndex(s[:idx], "\x1b[")
	if start < 0 {
		return ""
	}
	return s[start:idx]
}
