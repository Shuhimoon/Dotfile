package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const footerMaxLines = 3

// footerEllipsis is one space plus U+2026. ansi.StringWidth is 2.
const footerEllipsis = " …"

type hint struct {
	Key  string
	Desc string
}

type footerSpan struct {
	idxs []int
	ell  bool
	cut  bool
	hard string
}

const homeHelp = "操作  ↑/k 上一項  ↓/j 下一項  enter 進入  q 離開"

func footerHints(m model) []hint {
	switch m.phase {
	case phaseDetecting, phaseHome:
		return []hint{
			{Key: "操作"},
			{Key: "↑/k", Desc: "上一項"},
			{Key: "↓/j", Desc: "下一項"},
			{Key: "enter", Desc: "進入"},
			{Key: "q", Desc: "離開"},
		}
	case phaseUninstall:
		hints := []hint{{Key: "操作"}}
		for _, b := range m.keys.uninstalling() {
			h := b.Help()
			hints = append(hints, hint{Key: h.Key, Desc: h.Desc})
		}
		return hints
	case phaseSudo:
		return []hint{{Key: "操作"}, {Key: "enter", Desc: "確認"}, {Key: "esc", Desc: "返回"}}
	case phaseGitKey:
		return gitKeyHints(m)
	case phaseInstalling:
		return []hint{{Key: "操作"}, {Key: "ctrl+c", Desc: "取消"}, {Key: "↑/↓", Desc: "捲動清單"}, {Key: "end", Desc: "跟隨進度"}}
	case phaseReport:
		if m.dryRun {
			return []hint{{Key: "操作"}, {Key: "任意鍵", Desc: dryBackDesc(m)}}
		}
		if m.gitPending {
			return []hint{{Key: "操作"}, {Key: "任意鍵", Desc: "設定 GitHub SSH 鑰匙"}}
		}
		return []hint{{Key: "操作"}, {Key: "任意鍵", Desc: "回主頁"}}
	default:
		hints := []hint{{Key: "操作"}}
		for _, b := range m.keys.browsing() {
			h := b.Help()
			hints = append(hints, hint{Key: h.Key, Desc: h.Desc})
		}
		return hints
	}
}

func hintText(h hint) string {
	if h.Desc == "" {
		return h.Key
	}
	// reportHelp is "任意鍵回主頁" with no space between the key and the label.
	if h.Key == "任意鍵" && h.Desc == "回主頁" {
		return h.Key + h.Desc
	}
	return h.Key + " " + h.Desc
}

func footerSegments(m model) []string {
	hints := footerHints(m)
	segs := make([]string, len(hints))
	for i, h := range hints {
		segs[i] = hintText(h)
	}
	return segs
}

func footerLineCount(m model, styled bool) int {
	if !styled {
		return 1
	}
	lines, _ := layoutFooter(footerSegments(m), m.width, footerMaxLines)
	if len(lines) == 0 {
		return 1
	}
	return len(lines)
}

func layoutFooter(segments []string, width, maxLines int) (lines []string, truncated bool) {
	spans, truncated := layoutFooterSpans(segments, width, maxLines)
	lines = make([]string, len(spans))
	for i, sp := range spans {
		lines[i] = spanText(segments, sp)
	}
	return lines, truncated
}

func layoutFooterSpans(segments []string, width, maxLines int) ([]footerSpan, bool) {
	if maxLines < 1 || len(segments) == 0 {
		return nil, false
	}
	if width < 1 {
		return []footerSpan{{cut: true}}, true
	}
	rest := make([]int, len(segments))
	for i := range segments {
		rest[i] = i
	}
	var spans []footerSpan
	truncated := false
	for len(rest) > 0 && len(spans) < maxLines {
		last := len(spans) == maxLines-1
		if !last {
			n := countFitIdx(segments, rest, width)
			if n == 0 {
				spans = append(spans, footerSpan{cut: true, hard: ansi.Truncate(segments[rest[0]], width, "…")})
				rest = rest[1:]
				truncated = true
				continue
			}
			spans = append(spans, footerSpan{idxs: append([]int(nil), rest[:n]...)})
			rest = rest[n:]
			continue
		}
		if countFitIdx(segments, rest, width) == len(rest) {
			spans = append(spans, footerSpan{idxs: append([]int(nil), rest...)})
			rest = nil
			break
		}
		budget := width - ansi.StringWidth(footerEllipsis)
		if budget < 0 {
			budget = 0
		}
		n := countFitIdx(segments, rest, budget)
		if n == 0 {
			spans = append(spans, footerSpan{cut: true, hard: ansi.Truncate(segments[rest[0]], width, "…")})
			truncated = true
			rest = nil
			break
		}
		spans = append(spans, footerSpan{idxs: append([]int(nil), rest[:n]...), ell: true})
		truncated = true
		rest = nil
		break
	}
	if len(rest) > 0 {
		truncated = true
	}
	return spans, truncated
}

func spanText(segments []string, sp footerSpan) string {
	if sp.cut {
		return sp.hard
	}
	parts := make([]string, len(sp.idxs))
	for i, idx := range sp.idxs {
		parts[i] = segments[idx]
	}
	s := strings.Join(parts, "  ")
	if sp.ell {
		s += footerEllipsis
	}
	return s
}

func countFitIdx(segments []string, idxs []int, budget int) int {
	if budget <= 0 {
		return 0
	}
	w := 0
	n := 0
	for _, idx := range idxs {
		add := ansi.StringWidth(segments[idx])
		if n > 0 {
			add += 2
		}
		if w+add > budget {
			break
		}
		w += add
		n++
	}
	return n
}

func plainHelp(m model) string {
	switch m.phase {
	case phaseDetecting, phaseHome:
		return homeHelp
	case phaseUninstall:
		return uninstallHelp(m.keys)
	case phaseSudo:
		return sudoHelp
	case phaseGitKey:
		return gitKeyHelp(m)
	case phaseInstalling:
		return installingHelp
	case phaseReport:
		if m.dryRun {
			return "操作  任意鍵 " + dryBackDesc(m)
		}
		if m.gitPending {
			return "操作  任意鍵 設定 GitHub SSH 鑰匙"
		}
		return reportHelp
	default:
		return browsingHelp(m.keys)
	}
}

func paintHomeFooter(p palette, m model, footerN int) []string {
	if footerN <= 0 {
		return nil
	}
	hints := footerHints(m)
	spans, _ := layoutFooterSpans(footerSegments(m), m.width, footerN)
	out := make([]string, 0, len(spans))
	for _, sp := range spans {
		out = append(out, paintHomeFooterSpan(p, hints, sp, m.width))
	}
	return out
}

func paintHomeHint(p palette, h hint) string {
	key := homeFG(p, h.Key, homeKeyColor, false)
	if h.Desc == "" {
		return key
	}
	return key + " " + homeFG(p, h.Desc, textColor, false)
}

func paintHomeFooterSpan(p palette, hints []hint, sp footerSpan, width int) string {
	if sp.cut {
		if sp.hard == "" {
			return plainPad(0, "", width)
		}
		return plainPad(0, homeFG(p, sp.hard, textColor, false), width)
	}
	var b strings.Builder
	for i, idx := range sp.idxs {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(paintHomeHint(p, hints[idx]))
	}
	if sp.ell {
		b.WriteString(homeFG(p, footerEllipsis, textColor, false))
	}
	return plainPad(0, b.String(), width)
}
