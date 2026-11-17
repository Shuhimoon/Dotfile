package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/install"
)

func TestFooterIsLastLine(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	lines := frameLines(render(m, false))
	if len(lines) != 40 {
		t.Fatalf("lines %d", len(lines))
	}
	if lines[1] != "  基礎" {
		t.Fatalf("line 2 %q", lines[1])
	}
	narrow := New(fedoraInfo())
	narrow.width = 4
	narrow.height = 40
	nlines := frameLines(render(narrow, false))
	if len(nlines) != 40 || nlines[1] != "基礎" {
		t.Fatalf("width 4 line 2 %q", nlines[1])
	}
	help := browsingHelp(m.keys)
	if lines[len(lines)-1] != help {
		t.Fatalf("last %q", lines[len(lines)-1])
	}
	if lines[1] == help {
		t.Fatal("help is still under the header")
	}

	m.phase = phaseInstalling
	lines = frameLines(render(m, false))
	if lines[len(lines)-1] != installingHelp {
		t.Fatalf("installing %q", lines[len(lines)-1])
	}
	m.phase = phaseReport
	m.results = []install.ItemResult{{ID: "git", Status: "ok", Note: "note"}}
	lines = frameLines(render(m, false))
	if lines[len(lines)-1] != reportHelp {
		t.Fatalf("report %q", lines[len(lines)-1])
	}
}

func TestNarrowFooterStaysOnScreen(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 3
	lines := frameLines(render(m, false))
	if len(lines) != 3 {
		t.Fatalf("lines %d %q", len(lines), lines)
	}
	if lines[len(lines)-1] != browsingHelp(m.keys) {
		t.Fatalf("last %q", lines[len(lines)-1])
	}
	if strings.Contains(lines[0], "操作") {
		t.Fatal("help replaced the header")
	}
}

func TestStyledShortFrameClampsFooter(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	r := testRenderer()

	m.height = 1
	lines := frameLines(renderStyled(m, r))
	if len(lines) != 1 {
		t.Fatalf("h1 %d %q", len(lines), lines)
	}
	if strings.Contains(ansi.Strip(lines[0]), "系統:") || !strings.Contains(lines[0], "…") {
		t.Fatalf("h1 %q", lines[0])
	}

	m.height = 2
	view := renderStyled(m, r)
	lines = frameLines(view)
	if len(lines) != 2 {
		t.Fatalf("h2 %d", len(lines))
	}
	if strings.Contains(ansi.Strip(view), "系統:") || !strings.Contains(view, "離開") {
		t.Fatal("height 2 frame")
	}

	m.height = 3
	view = renderStyled(m, r)
	lines = frameLines(view)
	if len(lines) != 3 {
		t.Fatalf("h3 %d", len(lines))
	}
	if !strings.Contains(ansi.Strip(view), "系統:") {
		t.Fatal("height 3 missing header")
	}
	// 安裝已勾選 sits in the footer, so the status row is the line with 游標.
	if strings.Contains(view, "游標") {
		t.Fatal("height 3 drew status")
	}
}

func TestCatalogScrollKeepsCursor(t *testing.T) {
	m := New(fedoraInfo())
	m.width = 80
	m.height = 10
	for i, id := range m.flat {
		if id == "pkg-config" {
			m.cursor = i
		}
	}
	view := render(m, false)
	lines := frameLines(view)
	if !strings.Contains(view, "pkg-config") || !strings.Contains(view, ">") {
		t.Fatalf("cursor missing\n%s", view)
	}
	if lines[len(lines)-1] != browsingHelp(m.keys) {
		t.Fatalf("last %q", lines[len(lines)-1])
	}
	if strings.Contains(view, "基礎") {
		t.Fatal("scrolled-away header still visible")
	}
}

func TestProgressListWindowShowsRemainder(t *testing.T) {
	const n = 50
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	m.phase = phaseInstalling
	m.steps = make([]install.Step, n)
	for i := range m.steps {
		m.steps[i].Title = fmt.Sprintf("步驟%02d", i)
	}
	m.stepState = make([]string, n)
	m.applyProgress("步驟00", "")

	lines := frameLines(render(m, false))
	if len(lines) != 40 {
		t.Fatalf("plain lines %d", len(lines))
	}
	if lines[len(lines)-1] != installingHelp {
		t.Fatalf("footer %q", lines[len(lines)-1])
	}
	plain := strings.Join(lines, "\n")
	if !strings.Contains(plain, "安裝中  1/50  步驟00") || !strings.Contains(plain, "執行中  步驟00") {
		t.Fatalf("head/running row:\n%s", plain)
	}
	if !strings.Contains(plain, "…還有") || strings.Contains(plain, "步驟49") {
		t.Fatalf("list window:\n%s", plain)
	}

	view := renderStyled(m, testRenderer())
	slines := frameLines(view)
	if len(slines) != 40 {
		t.Fatalf("styled lines %d", len(slines))
	}
	if strings.Contains(view, "48;2;") || strings.Contains(view, "步驟49") {
		t.Fatal("styled window or background")
	}
	if !strings.Contains(slines[len(slines)-1], "ctrl+c") {
		t.Fatalf("styled footer %q", slines[len(slines)-1])
	}
}

func TestFooterLayoutWidth(t *testing.T) {
	m := New(fedoraInfo())
	segs := footerSegments(m)
	full := browsingHelp(m.keys)
	if strings.Join(segs, "  ") != full {
		t.Fatalf("segments %q", strings.Join(segs, "  "))
	}
	if strings.Join(footerSegments(phaseModel(m, phaseSudo)), "  ") != sudoHelp {
		t.Fatal("sudo segments")
	}
	if strings.Join(footerSegments(phaseModel(m, phaseInstalling)), "  ") != installingHelp {
		t.Fatal("installing segments")
	}
	if strings.Join(footerSegments(phaseModel(m, phaseReport)), "  ") != reportHelp {
		t.Fatal("report segments")
	}

	checkFit := func(width, n int) []string {
		t.Helper()
		lines, truncated := layoutFooter(segs, width, footerMaxLines)
		if truncated {
			t.Fatalf("width %d truncated %q", width, lines)
		}
		if len(lines) != n {
			t.Fatalf("width %d lines %d %q", width, len(lines), lines)
		}
		if strings.Join(lines, "  ") != full {
			t.Fatalf("width %d join %q", width, strings.Join(lines, "  "))
		}
		for _, ln := range lines {
			if ansi.StringWidth(ln) > width {
				t.Fatalf("width %d overflow %d %q", width, ansi.StringWidth(ln), ln)
			}
		}
		return lines
	}

	one := checkFit(141, 1)
	if one[0] != full {
		t.Fatalf("141 %q", one[0])
	}
	two := checkFit(80, 2)
	if !strings.HasSuffix(two[0], "a 切換本類") {
		t.Fatalf("80 first %q", two[0])
	}
	checkFit(79, 2)
	checkFit(63, 3)
	checkFit(60, 3)

	for _, width := range []int{44, 40} {
		lines, truncated := layoutFooter(segs, width, footerMaxLines)
		if !truncated || len(lines) != 3 {
			t.Fatalf("width %d trunc %v %q", width, truncated, lines)
		}
		last := lines[len(lines)-1]
		if !strings.HasSuffix(last, " …") {
			t.Fatalf("width %d last %q", width, last)
		}
		if ansi.StringWidth(last) > width {
			t.Fatalf("width %d last width %d %q", width, ansi.StringWidth(last), last)
		}
		joined := strings.Join(lines, "\n")
		if strings.Contains(joined, "q 離開") || strings.Contains(joined, "g 頂端") {
			t.Fatalf("width %d kept tail %q", width, lines)
		}
		if last != "a 切換本類  A 切換全部  d 只顯示計畫 …" {
			t.Fatalf("width %d last %q", width, last)
		}
	}
}

func phaseModel(m model, ph phase) model {
	m.phase = ph
	return m
}

func frameLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
