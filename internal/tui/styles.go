package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type palette struct {
	r         *lipgloss.Renderer
	bark      lipgloss.Color
	header    lipgloss.Color
	installed lipgloss.Color
	title     lipgloss.Color
	cursor    lipgloss.Color
	footer    lipgloss.Color
	mode      lipgloss.Color
	cream     lipgloss.Color
	clay      lipgloss.Color
	sand      lipgloss.Color
	gold      lipgloss.Color
	key       lipgloss.Color
	olive     lipgloss.Color
	muted     lipgloss.Color
}

func earthStyles(r *lipgloss.Renderer) palette {
	// tools-download and tools-uninstall share the home page theme: text colours
	// from the title gradient #F0ADA0 -> #9EE2F0 and no background colour at all;
	// the terminal background shows through. The cursor row is the ">" marker plus
	// peach text. bark, header, installed, title, cursor, footer and mode are
	// background tokens that ink/gap/padTo still accept but ignore.
	return palette{
		r:         r,
		bark:      "#1E1A1D",
		header:    "#3A2C30",
		installed: "#1D2E33",
		title:     "#2B2226",
		cursor:    selectColor,
		footer:    "#2B2226",
		mode:      "#1E1A1D",
		cream:     textColor,
		clay:      selectColor,
		sand:      "#D9CFCD",
		gold:      headColor,
		key:       homeKeyColor,
		olive:     headColor,
		muted:     "#9C8F92",
	}
}

// catColor shades category headers along the title gradient, top to bottom.
func catColor(i, n int) lipgloss.Color {
	return lipgloss.Color(lerpTitle(n, i))
}

func (p palette) ink(text string, fg, bg lipgloss.Color, bold bool) string {
	// No background colour anywhere: bg is kept in the signature but ignored.
	st := p.r.NewStyle().Inline(true).Foreground(fg)
	if bold {
		st = st.Bold(true)
	}
	return st.Render(text)
}

func (p palette) gap(text string, bg lipgloss.Color) string {
	return text
}

func (p palette) padTo(s string, width int, bg lipgloss.Color) string {
	if width <= 0 {
		return s
	}
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + p.gap(strings.Repeat(" ", width-w), bg)
}

func (p palette) blank(width int) string {
	if width <= 0 {
		return ""
	}
	// Plain spaces: no screen paints a background colour.
	return strings.Repeat(" ", width)
}

func paintHeader(p palette, text string, width int, bold bool) string {
	return paintBar(p, text, width, p.header, bold)
}

func paintTitle(p palette, text string, width int) string {
	visible, left := placeHeader(text, width)
	var b strings.Builder
	if left > 0 {
		b.WriteString(p.gap(strings.Repeat(" ", left), p.title))
	}
	rs := []rune(visible)
	for i, r := range rs {
		b.WriteString(p.ink(string(r), lipgloss.Color(lerpTitle(len(rs), i)), p.title, true))
	}
	return p.padTo(b.String(), width, p.title)
}

func paintBar(p palette, text string, width int, bg lipgloss.Color, bold bool) string {
	visible, left := placeHeader(text, width)
	var b strings.Builder
	if left > 0 {
		b.WriteString(p.gap(strings.Repeat(" ", left), bg))
	}
	if visible != "" {
		b.WriteString(p.ink(visible, p.cream, bg, bold))
	}
	return p.padTo(b.String(), width, bg)
}

func catalogBlank(p palette, width int) string {
	inset := horizontalInset(width)
	inner := width - 2*inset
	if inner < 0 {
		inner = 0
	}
	var mid string
	if inner > 0 {
		mid = p.gap(strings.Repeat(" ", inner), p.bark)
	}
	return strings.Repeat(" ", inset) + mid + strings.Repeat(" ", inset)
}

func paintItem(p palette, row catalogRow, width int, showDesc, installed bool) string {
	inset := horizontalInset(width)
	inner := width - 2*inset
	if inner <= 0 {
		if width <= 0 {
			return ""
		}
		return strings.Repeat(" ", width)
	}
	if installed {
		return paintInstalledItem(p, row, width, inset, inner)
	}
	bg := p.bark
	if row.Cursor {
		bg = p.cursor
	}
	on := cursorInk(row.Cursor)
	fg := p.sand
	box := "[ ]"
	if row.Selected {
		fg = p.clay
		box = "[x]"
	}
	name := row.Item.Name
	padN := catalogNameWidth() - ansi.StringWidth(name)
	if padN < 0 {
		padN = 0
	}

	var b strings.Builder
	add := func(styled string) {
		b.WriteString(styled)
	}
	if row.Cursor {
		add(p.ink(">", on(p.cream), bg, true))
		add(p.gap(" ", bg))
	} else {
		add(p.gap("  ", bg))
	}
	if row.Item.Default {
		add(p.ink("*", on(p.gold), bg, false))
	} else {
		add(p.gap(" ", bg))
	}
	add(p.gap(" ", bg))
	add(p.ink(box, on(fg), bg, false))
	add(p.gap(" ", bg))
	add(p.ink(name, on(fg), bg, false))
	if padN > 0 {
		add(p.gap(strings.Repeat(" ", padN), bg))
	}
	if showDesc && row.Item.Description != "" {
		add(p.ink("  ", on(p.muted), bg, false))
		add(p.ink(row.Item.Description, on(p.muted), bg, false))
	}

	colored := b.String()
	if ansi.StringWidth(colored) > inner {
		colored = ansi.Truncate(colored, inner, "…")
	}
	if w := ansi.StringWidth(colored); w < inner {
		colored += p.gap(strings.Repeat(" ", inner-w), bg)
	}
	return strings.Repeat(" ", inset) + colored + strings.Repeat(" ", inset)
}

func paintInstalledItem(p palette, row catalogRow, width, inset, inner int) string {
	bg := p.installed
	if row.Cursor {
		bg = p.cursor
	}
	on := cursorInk(row.Cursor)
	fg := p.sand
	box := "[ ]"
	if row.Selected {
		fg = p.clay
		box = "[x]"
	}
	name := row.Item.Name
	padN := catalogNameWidth() - ansi.StringWidth(name)
	if padN < 0 {
		padN = 0
	}

	var styled strings.Builder
	add := func(st string) { styled.WriteString(st) }
	if row.Cursor {
		add(p.ink(">", on(p.cream), bg, true))
		add(p.gap(" ", bg))
	} else {
		add(p.gap("  ", bg))
	}
	if row.Item.Default {
		add(p.ink("*", on(p.gold), bg, false))
	} else {
		add(p.gap(" ", bg))
	}
	add(p.gap(" ", bg))
	add(p.ink(box, on(fg), bg, false))
	add(p.gap(" ", bg))
	add(p.ink(name, on(fg), bg, false))
	if padN > 0 {
		add(p.gap(strings.Repeat(" ", padN), bg))
	}
	if width >= 100 && row.Item.Description != "" {
		add(p.ink("  ", on(p.sand), bg, false))
		add(p.ink(row.Item.Description, on(p.sand), bg, false))
	}

	left := itemLeft(width, row)
	visible, gap, mark := placeInstalled(left, inner)
	var colored string
	if inner < 8 {
		if visible != "" {
			colored += p.ink(visible, on(fg), bg, false)
		}
		if mark != "" {
			colored += p.ink(mark, on(p.key), bg, false)
		}
	} else {
		body := styled.String()
		if left != visible {
			body = ansi.Truncate(body, ansi.StringWidth(visible), "…")
		}
		if short := ansi.StringWidth(visible) - ansi.StringWidth(body); short > 0 {
			body += p.gap(strings.Repeat(" ", short), bg)
		}
		colored = body
		if gap != "" {
			colored += p.gap(gap, bg)
		}
		if mark != "" {
			colored += p.ink(mark, on(p.key), bg, false)
		}
	}
	return strings.Repeat(" ", inset) + colored + strings.Repeat(" ", inset)
}

func paintCatalogHeader(p palette, title string, width int, fg lipgloss.Color) string {
	inset := horizontalInset(width)
	inner := width - 2*inset
	if inner < 0 {
		inner = 0
	}
	text := title
	if inner == 0 {
		text = ""
	} else if ansi.StringWidth(title) > inner {
		text = ansi.Truncate(title, inner, "…")
	}
	var body string
	if inner > 0 {
		body = p.padTo(p.ink(text, fg, p.bark, true), inner, p.bark)
	}
	return strings.Repeat(" ", inset) + body + strings.Repeat(" ", inset)
}

func paintBodyLine(p palette, text string, width int) string {
	return p.padTo(p.ink(text, p.cream, p.bark, false), width, p.bark)
}

func paintStatusLine(p palette, text string, fg lipgloss.Color, width int) string {
	return p.padTo(p.ink(text, fg, p.bark, false), width, p.bark)
}

func paintHint(p palette, h hint) string {
	if h.Key == "操作" && h.Desc == "" {
		return p.ink(h.Key, p.cream, p.mode, false)
	}
	var b strings.Builder
	b.WriteString(p.ink(h.Key, p.key, p.footer, false))
	if h.Desc == "" {
		return b.String()
	}
	if !(h.Key == "任意鍵" && h.Desc == "返回") {
		b.WriteString(p.gap(" ", p.footer))
	}
	b.WriteString(p.ink(h.Desc, p.cream, p.footer, false))
	return b.String()
}

func paintFooterSpan(p palette, hints []hint, sp footerSpan, width int) string {
	if sp.cut {
		if sp.hard == "" {
			return p.padTo("", width, p.footer)
		}
		return p.padTo(p.ink(sp.hard, p.cream, p.footer, false), width, p.footer)
	}
	var b strings.Builder
	for i, idx := range sp.idxs {
		if i > 0 {
			b.WriteString(p.gap("  ", p.footer))
		}
		b.WriteString(paintHint(p, hints[idx]))
	}
	if sp.ell {
		b.WriteString(p.ink(footerEllipsis, p.cream, p.footer, false))
	}
	return p.padTo(b.String(), width, p.footer)
}

func paintFooter(p palette, m model, footerN int) []string {
	if footerN <= 0 {
		return nil
	}
	hints := footerHints(m)
	spans, _ := layoutFooterSpans(footerSegments(m), m.width, footerN)
	out := make([]string, 0, len(spans))
	for _, sp := range spans {
		out = append(out, paintFooterSpan(p, hints, sp, m.width))
	}
	return out
}

// cursorInk paints the whole cursor row in the peach select colour (no fill).
func cursorInk(cursor bool) func(lipgloss.Color) lipgloss.Color {
	return func(c lipgloss.Color) lipgloss.Color {
		if cursor {
			return selectColor
		}
		return c
	}
}
