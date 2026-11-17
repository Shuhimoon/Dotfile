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
	// header is only the force-reinstall bar.
	return palette{
		r:         r,
		bark:      "#2B2D3A",
		header:    "#3F445B",
		installed: "#423F59",
		title:     "#394934",
		cursor:    "#333648",
		footer:    "#393E53",
		mode:      "#2B2D3A",
		cream:     "#E1E3E4",
		clay:      "#F89860",
		sand:      "#E1E3E4",
		gold:      "#EDC763",
		key:       "#6DCAE8",
		olive:     "#9ED06C",
		muted:     "#7E8294",
	}
}

func (p palette) ink(text string, fg, bg lipgloss.Color, bold bool) string {
	st := p.r.NewStyle().Inline(true).Foreground(fg).Background(bg)
	if bold {
		st = st.Bold(true)
	}
	return st.Render(text)
}

func (p palette) gap(text string, bg lipgloss.Color) string {
	return p.r.NewStyle().Inline(true).Background(bg).Render(text)
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
	// Empty string, so Width paints the spaces itself. A precolored row must not go through Width.
	return p.r.NewStyle().Background(p.bark).Width(width).Render("")
}

func paintHeader(p palette, text string, width int, bold bool) string {
	return paintBar(p, text, width, p.header, bold)
}

func paintTitle(p palette, text string, width int) string {
	return paintBar(p, text, width, p.title, true)
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
		add(p.ink(">", p.cream, bg, true))
		add(p.gap(" ", bg))
	} else {
		add(p.gap("  ", bg))
	}
	if row.Item.Default {
		add(p.ink("*", p.gold, bg, false))
	} else {
		add(p.gap(" ", bg))
	}
	add(p.gap(" ", bg))
	add(p.ink(box, fg, bg, false))
	add(p.gap(" ", bg))
	add(p.ink(name, fg, bg, false))
	if padN > 0 {
		add(p.gap(strings.Repeat(" ", padN), bg))
	}
	if showDesc && row.Item.Description != "" {
		add(p.ink("  ", p.muted, bg, false))
		add(p.ink(row.Item.Description, p.muted, bg, false))
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
		add(p.ink(">", p.cream, bg, true))
		add(p.gap(" ", bg))
	} else {
		add(p.gap("  ", bg))
	}
	if row.Item.Default {
		add(p.ink("*", p.gold, bg, false))
	} else {
		add(p.gap(" ", bg))
	}
	add(p.gap(" ", bg))
	add(p.ink(box, fg, bg, false))
	add(p.gap(" ", bg))
	add(p.ink(name, fg, bg, false))
	if padN > 0 {
		add(p.gap(strings.Repeat(" ", padN), bg))
	}
	if width >= 100 && row.Item.Description != "" {
		add(p.ink("  ", p.sand, bg, false))
		add(p.ink(row.Item.Description, p.sand, bg, false))
	}

	left := itemLeft(width, row)
	visible, gap, mark := placeInstalled(left, inner)
	var colored string
	if inner < 8 {
		if visible != "" {
			colored += p.ink(visible, fg, bg, false)
		}
		if mark != "" {
			colored += p.ink(mark, fg, bg, false)
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
			colored += p.ink(mark, fg, bg, false)
		}
	}
	return strings.Repeat(" ", inset) + colored + strings.Repeat(" ", inset)
}

func paintCatalogHeader(p palette, title string, width int) string {
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
		body = p.padTo(p.ink(text, p.olive, p.bark, true), inner, p.bark)
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
