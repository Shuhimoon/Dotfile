package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bigGlyphs draws each title letter three rows tall with heavy box-drawing lines.
var bigGlyphs = map[rune][3]string{
	'T': {"╺━┳━╸", "  ┃  ", "  ╹  "},
	'O': {"┏━━━┓", "┃   ┃", "┗━━━┛"},
	'L': {"╻    ", "┃    ", "┗━━━╸"},
	'S': {"┏━━━╸", "┗━━━┓", "╺━━━┛"},
	'-': {"   ", "╺━╸", "   "},
	'I': {"╺┳╸", " ┃ ", "╺┻╸"},
	'N': {"┏┓  ╻", "┃┗┓ ┃", "╹ ┗━┛"},
	'A': {"┏━━━┓", "┣━━━┫", "╹   ╹"},
}

const bigTitleText = "TOOLS-INSTALL"

// bigStrokes lists every character the big title can draw with.
const bigStrokes = "━┃┏┓┗┛┳┻┣┫╸╺╹╻"

// bigTitleRows lays the glyphs side by side with a one-column gap.
func bigTitleRows() []string {
	var rows [3]strings.Builder
	for gi, r := range []rune(bigTitleText) {
		g := bigGlyphs[r]
		for i := range rows {
			if gi > 0 {
				rows[i].WriteByte(' ')
			}
			rows[i].WriteString(g[i])
		}
	}
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = rows[i].String()
	}
	return out
}

func bigTitleWidth() int {
	return len([]rune(bigTitleRows()[0]))
}

// titleLines picks the big title when asked and when it fits; otherwise the one-line title.
func titleLines(w int, big bool) []hLine {
	if !big || w < bigTitleWidth()+4 {
		return buildTitle(w)
	}
	rows := bigTitleRows()
	left := (w - bigTitleWidth()) / 2
	out := make([]hLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, hLine{kind: hBig, text: r, left: left})
	}
	return out
}

// paintBigRow shades every column along the title gradient, left to right.
func paintBigRow(p palette, ln hLine, width int) string {
	if width <= 0 {
		return ""
	}
	n := bigTitleWidth()
	var b strings.Builder
	for x, c := range []rune(ln.text) {
		if c == ' ' {
			b.WriteRune(c)
			continue
		}
		b.WriteString(homeFG(p, string(c), lipgloss.Color(lerpTitle(n, x)), true))
	}
	return plainPad(ln.left, b.String(), width)
}
