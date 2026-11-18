package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/platform"
)

// Windows console quirks.
//
// The classic console host (conhost, used by cmd.exe and Windows PowerShell
// outside Windows Terminal) differs from the terminals the layout is measured
// for in two ways:
//
//  1. Without DISABLE_NEWLINE_AUTO_RETURN it moves the cursor to the next row
//     as soon as a character lands in the last column. Every line here is
//     padded to the full width, so each one wrapped an extra time and Bubble
//     Tea's "cursor up N rows, repaint changed rows" redraw landed on the wrong
//     rows: old rows stayed on screen and the menu showed duplicates.
//  2. With a CJK code page (zh-TW is 950) it draws East Asian "ambiguous"
//     characters such as ╭─│ … ↑ two cells wide, while every width function
//     here counts them as one, so the card rows wrapped too.
//
// ReserveLastColumn lays everything out one column narrower, and ASCII maps
// the ambiguous symbols to one-cell ASCII before the frame is written.

// asciiMap replaces each ambiguous-width symbol the UI draws with a
// one-cell ASCII character, so widths stay the same.
var asciiMap = map[rune]rune{
	'…': '.', '↑': '^', '↓': 'v', '←': '<', '→': '>',
	'ö': 'o', 'ä': 'a', '•': '*', '·': '.', '×': 'x',
	'✓': 'v', '✗': 'x', '●': '*', '○': 'o', '▸': '>', '►': '>',
}

// boxASCII maps U+2500–U+257F box drawing to - | or +.
func boxASCII(r rune) (rune, bool) {
	if r < 0x2500 || r > 0x257F {
		return 0, false
	}
	switch r {
	case '─', '━', '┄', '┅', '┈', '┉', '╌', '╍', '═', '╴', '╶', '╸', '╺', '╼', '╾':
		return '-', true
	case '│', '┃', '┆', '┇', '┊', '┋', '╎', '╏', '║', '╵', '╷', '╹', '╻', '╽', '╿':
		return '|', true
	default:
		return '+', true
	}
}

// ASCIIFrame rewrites a rendered frame for a console that draws ambiguous
// characters wide. ANSI escape sequences are ASCII and pass through.
func ASCIIFrame(s string) string {
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if a, ok := asciiMap[r]; ok {
			b.WriteRune(a)
			continue
		}
		if a, ok := boxASCII(r); ok {
			b.WriteRune(a)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// layoutWidth is the width the UI lays out for a terminal of w columns.
func (m model) layoutWidth(w int) int {
	if m.deps.ReserveLastColumn && w > 1 {
		return w - 1
	}
	return w
}

func archFamily(info platform.Info) string {
	if info.Arch.Family == "" {
		return "unknown"
	}
	return info.Arch.Family
}

// clipFrame cuts every line to w cells so nothing reaches the reserved
// last column, whatever a page forgot to truncate.
func clipFrame(s string, w int) string {
	if w <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ansi.StringWidth(ln) > w {
			lines[i] = ansi.Truncate(ln, w, "")
		}
	}
	return strings.Join(lines, "\n")
}
