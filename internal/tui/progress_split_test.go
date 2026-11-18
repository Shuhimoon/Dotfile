package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/install"
)

func TestListWindowKeepsFocusVisible(t *testing.T) {
	const n = 30
	for rows := 1; rows <= 16; rows++ {
		for focus := 0; focus < n; focus++ {
			start, end, above, below := listWindow(n, focus, rows, 0, false)
			if focus < start || focus >= end {
				t.Fatalf("rows %d focus %d not in [%d,%d)", rows, focus, start, end)
			}
			used := end - start
			if above > 0 && rows >= 3 {
				used++
			}
			if below > 0 && rows >= 3 {
				used++
			}
			if used > rows {
				t.Fatalf("rows %d focus %d uses %d rows", rows, focus, used)
			}
			if above != start || below != n-end {
				t.Fatalf("hint counts %d %d for [%d,%d)", above, below, start, end)
			}
			if rows >= 6 && focus+2 < n && end < focus+3 {
				t.Fatalf("rows %d focus %d: next steps hidden (end %d)", rows, focus, end)
			}
		}
	}
	if s, e, _, _ := listWindow(10, 3, 20, 0, false); s != 0 || e != 10 {
		t.Fatalf("short list windowed: %d %d", s, e)
	}
}

func runningModel(t *testing.T, n, width, height, running int) model {
	t.Helper()
	m := New(fedoraInfo())
	m.width, m.height = width, height
	m.phase = phaseInstalling
	m.mode = modeInstall
	m.steps = make([]install.Step, n)
	for i := range m.steps {
		m.steps[i].Title = fmt.Sprintf("install tool%02d", i)
	}
	m.stepState = make([]string, n)
	for i := 0; i < running; i++ {
		m.applyProgress(m.steps[i].Title, "")
		m.applyProgress(m.steps[i].Title, "ok")
	}
	m.applyProgress(m.steps[running].Title, "")
	for i := 0; i < 50; i++ {
		m.appendOutput(fmt.Sprintf("輸出第 %d 行 Downloading packages 下載中 %s", i, strings.Repeat("x", i)), false)
	}
	return m
}

func TestProgressSplitFollowsRunningStep(t *testing.T) {
	for _, width := range []int{60, 80, 120, 200} {
		for _, height := range []int{8, 12, 20, 40} {
			for _, run := range []int{0, 7, 17, 29} {
				m := runningModel(t, 30, width, height, run)
				for _, styled := range []bool{false, true} {
					var view string
					if styled {
						view = renderStyled(m, testRenderer())
					} else {
						view = render(m, false)
					}
					lines := frameLines(view)
					if len(lines) != height {
						t.Fatalf("w%d h%d run%d styled=%v: %d lines", width, height, run, styled, len(lines))
					}
					for i, ln := range lines {
						if w := ansi.StringWidth(ln); w > width {
							t.Fatalf("w%d h%d run%d styled=%v line %d is %d wide: %q", width, height, run, styled, i, w, ansi.Strip(ln))
						}
					}
					plain := ansi.Strip(view)
					cur := fmt.Sprintf("執行中  install tool%02d", run)
					if height >= 12 && !strings.Contains(plain, cur) {
						t.Fatalf("w%d h%d run%d styled=%v: running step hidden\n%s", width, height, run, styled, plain)
					}
					if height >= 12 && height <= 20 && run >= 17 && !strings.Contains(plain, "…上方") {
						t.Fatalf("w%d h%d run%d: no 上方 hint\n%s", width, height, run, plain)
					}
				}
				if (width >= splitMinWidth) != m.usesSplit() {
					t.Fatalf("w%d split=%v", width, m.usesSplit())
				}
			}
		}
	}
}

func TestProgressSplitPanes(t *testing.T) {
	m := runningModel(t, 30, 120, 20, 18)
	view := render(m, false)
	lines := frameLines(view)
	var split []string
	for _, ln := range lines {
		if strings.Contains(ln, "│") {
			split = append(split, ln)
		}
	}
	if len(split) < 10 {
		t.Fatalf("not split\n%s", view)
	}
	col := ansi.StringWidth(split[0][:strings.Index(split[0], "│")])
	for _, ln := range split {
		if ansi.StringWidth(ln[:strings.Index(ln, "│")]) != col {
			t.Fatalf("divider not aligned\n%s", view)
		}
	}
	if !strings.Contains(split[0], "│ 輸出") || !strings.Contains(view, "輸出第 49 行") {
		t.Fatalf("output pane\n%s", view)
	}
	if !strings.HasPrefix(split[0], "…上方") {
		t.Fatalf("left pane top\n%s", view)
	}
	narrow := runningModel(t, 30, 70, 20, 18)
	nv := render(narrow, false)
	if strings.Contains(nv, "│") || !strings.Contains(nv, "\n輸出\n") || !strings.Contains(nv, "執行中  install tool18") {
		t.Fatalf("narrow layout\n%s", nv)
	}
}

func TestProgressManualScrollThenFollow(t *testing.T) {
	m := runningModel(t, 30, 100, 16, 20)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyHome})
	if !m.progManual {
		t.Fatal("home did not scroll")
	}
	view := render(m, false)
	if !strings.Contains(view, "install tool00") || strings.Contains(view, "執行中  install tool20") {
		t.Fatalf("manual top\n%s", view)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if v := render(m, false); strings.Contains(v, "install tool00") || !strings.Contains(v, "install tool01") {
		t.Fatalf("down\n%s", v)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.progManual || !strings.Contains(render(m, false), "執行中  install tool20") {
		t.Fatal("end did not resume follow")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if !m.progManual {
		t.Fatal("pgup did not scroll")
	}
	m.applyProgress("install tool20", "ok")
	m.applyProgress("install tool21", "")
	if m.progManual || !strings.Contains(render(m, false), "執行中  install tool21") {
		t.Fatal("next step did not resume follow")
	}
	if m.phase != phaseInstalling {
		t.Fatal("scroll keys left the progress page")
	}
}
