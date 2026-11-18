package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// The progress page is split vertically on wide terminals: the step list on
// the left, the command output on the right. Narrow terminals keep the
// stacked layout (list above, output below).

const (
	// splitMinWidth is the narrowest terminal that gets the side-by-side layout.
	splitMinWidth = 80
	splitLeftMin  = 28
	splitLeftMax  = 56
	splitDivider  = " │ "
)

// prow is one body row: a full-width line, or a left and a right cell.
type prow struct {
	split       bool
	full        pline
	left, right pline
}

// splitWidths is the left pane width and the right pane width (the divider
// takes the rest). ok is false when the terminal is too narrow to split.
// content is the widest list line: the left pane is as wide as that, between
// splitLeftMin and 45% of the terminal (at most splitLeftMax).
func splitWidths(width, content int) (left, right int, ok bool) {
	if width < splitMinWidth {
		return 0, 0, false
	}
	limit := min(width*45/100, splitLeftMax)
	left = max(splitLeftMin, min(content, limit))
	right = width - left - ansi.StringWidth(splitDivider)
	return left, right, right > 0
}

// usesSplit is true when the progress page is drawn side by side: a wide
// terminal, and something for the right pane (a run, or output after it).
func (m model) usesSplit() bool {
	if _, _, ok := splitWidths(m.width, 0); !ok {
		return false
	}
	if m.phase == phaseInstalling {
		return true
	}
	return m.phase == phaseReport && len(m.output) > 0
}

// listWindow picks rows of a list of n items for a pane of `rows` lines and
// reports how many items are hidden above and below. The hint rows ("…上方 N
// 項", "…還有 N 項") take a line each. Auto-follow keeps focus visible with up
// to two following items under it, so finished steps scroll off the top.
// manual uses offset (first item) instead, clamped to the list.
func listWindow(n, focus, rows, offset int, manual bool) (start, end, above, below int) {
	if n <= 0 || rows <= 0 {
		return 0, 0, 0, 0
	}
	if n <= rows {
		return 0, n, 0, 0
	}
	if focus < 0 {
		focus = 0
	}
	if focus >= n {
		focus = n - 1
	}
	topHint, botHint := 0, 1
	for i := 0; i < 3; i++ {
		c := rows - topHint - botHint
		if c < 1 {
			c = 1
		}
		if manual {
			start = offset
		} else {
			ahead := n - 1 - focus
			if ahead > 2 {
				ahead = 2
			}
			if ahead > c-1 {
				ahead = c - 1
			}
			start = focus - (c - 1 - ahead)
		}
		if start > n-c {
			start = n - c
		}
		if start < 0 {
			start = 0
		}
		end = start + c
		nt, nb := 0, 0
		if start > 0 {
			nt = 1
		}
		if end < n {
			nb = 1
		}
		if nt == topHint && nb == botHint {
			break
		}
		topHint, botHint = nt, nb
	}
	if rows-topHint-botHint < 1 {
		// One or two rows: show the focus row only.
		start, end = focus, focus+1
	}
	return start, end, start, n - end
}

// windowList returns the visible lines of list with the hint rows.
func windowList(list []pline, focus, rows, offset int, manual bool) []pline {
	start, end, above, below := listWindow(len(list), focus, rows, offset, manual)
	out := make([]pline, 0, rows)
	if above > 0 && rows-(end-start) >= 1 {
		out = append(out, pline{fmt.Sprintf("…上方 %d 項", above), toneMuted})
	}
	out = append(out, list[start:end]...)
	if below > 0 && len(out) < rows {
		out = append(out, pline{fmt.Sprintf("…還有 %d 項", below), toneMuted})
	}
	return out
}

// progressList is the left list (steps, results, or the plan) and its focus.
func (m model) progressList() ([]pline, int) {
	list := m.stepLines()
	focus := m.stepRun
	if focus < 0 {
		focus = m.stepNext
		if focus >= len(list) {
			focus = len(list) - 1
		}
	}
	if m.phase == phaseReport && m.dryRun {
		out := make([]pline, 0, len(m.plan))
		for _, s := range m.plan {
			out = append(out, pline{s, tonePlain})
		}
		return out, 0
	}
	if m.phase == phaseReport && len(m.results) > 0 {
		return m.resultLines(), 0
	}
	return list, focus
}

// progressTop is the head line and the wrapped warnings (full width).
func progressTop(m model) []pline {
	lines := []pline{m.progressHead()}
	for _, w := range m.warnings {
		// Warnings wrap instead of being cut: they are the only place the
		// python risk is shown now that nothing asks for y.
		for _, part := range strings.Split(ansi.Hardwrap(w, max(m.width, 1), true), "\n") {
			lines = append(lines, pline{part, toneWarn})
		}
	}
	return lines
}

// splitRows lays the side-by-side page out in bodyH rows or fewer.
func splitRows(m model, bodyH int) []prow {
	if bodyH <= 0 {
		return nil
	}
	var rows []prow
	for _, l := range progressTop(m) {
		rows = append(rows, prow{full: l})
	}
	if len(rows) >= bodyH {
		return rows[:bodyH]
	}
	paneH := bodyH - len(rows)
	list, focus := m.progressList()
	left := windowList(list, focus, paneH, m.progOffset, m.progManual)
	right := []pline{{"輸出", toneMuted}}
	if n := paneH - 1; n > 0 && len(m.output) > 0 {
		if n > len(m.output) {
			n = len(m.output)
		}
		for _, o := range m.output[len(m.output)-n:] {
			right = append(right, pline{o, toneOut})
		}
	}
	if len(right) > paneH {
		right = right[:paneH]
	}
	n := max(len(left), len(right))
	for i := 0; i < n; i++ {
		r := prow{split: true}
		if i < len(left) {
			r.left = left[i]
		}
		if i < len(right) {
			r.right = right[i]
		}
		rows = append(rows, r)
	}
	return rows
}

// splitLine joins a left and a right cell, each cut to its pane.
func splitLine(left, right string, lw, rw int) string {
	l := fitCell(left, lw)
	return padWidth(l, lw) + splitDivider + fitCell(right, rw)
}

// fitCell cuts s to w cells with a trailing "…" when it does not fit.
func fitCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

// progressScrollKey moves the left list by hand while a run is going.
// end/G returns to auto-follow; the next step starting does too.
func (m model) progressScrollKey(k tea.KeyMsg) (model, bool) {
	_, _, bodyH, _ := frameSize(m, m.deps.Styled)
	paneH := bodyH - len(progressTop(m))
	list, focus := m.progressList()
	if paneH <= 0 || len(list) <= paneH {
		return m, false
	}
	start, end, _, _ := listWindow(len(list), focus, paneH, m.progOffset, m.progManual)
	page := max(end-start-1, 1)
	off := start
	switch k.String() {
	case "up", "k":
		off--
	case "down", "j":
		off++
	case "pgup", "ctrl+b":
		off -= page
	case "pgdown", "ctrl+f", " ":
		off += page
	case "home", "g":
		off = 0
	case "end", "G":
		m.progManual = false
		m.progOffset = 0
		return m, true
	default:
		return m, false
	}
	off = max(0, min(off, len(list)-1))
	m.progManual = true
	m.progOffset = off
	return m, true
}

// paneWidths sizes the panes for the current list.
func (m model) paneWidths() (left, right int) {
	list, _ := m.progressList()
	content := 0
	for _, l := range list {
		content = max(content, ansi.StringWidth(strings.TrimRight(l.text, " ")))
	}
	left, right, _ = splitWidths(m.width, content)
	return left, right
}
