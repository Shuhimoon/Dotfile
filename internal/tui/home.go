package tui

import (
	"fmt"
	"math"
	"net"
	"os"
	"os/user"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/platform"
)

const (
	identityFallback = "未知"
	homeTitleText    = "Tools-install"
	itemDownload     = "tools-download"
	itemUninstall    = "tools-uninstall"
	hintDownload     = "進入目錄安裝"
	hintUninstall    = "此動作不可用"
	unavailLead      = "此動作不可用"
	unavailSentence  = "不移除發行版套件，也不刪除檔案。"

	titleStartHex = "#F0ADA0"

	// Colors below are taken from the title gradient (#F0ADA0 → #9EE2F0).
	selectColor  lipgloss.Color = "#F0ADA0"
	inkColor     lipgloss.Color = "#1E1416"
	textColor    lipgloss.Color = "#EDE3E1"
	headColor    lipgloss.Color = "#F7C6BA"
	homeKeyColor lipgloss.Color = "#9EE2F0"

	cardLabelW = 6
)

type identityMsg struct {
	User string
	Host string
	IP   string
	DNS  string
}

type hKind int

const (
	hTitle hKind = iota
	hBlank
	hCard
	hHead
	hMenu
	hNote
	hRule
	hBig
)

type hLine struct {
	kind     hKind
	text     string
	left     int
	selected bool
	mid      string
	hint     string
	label    string
	color    lipgloss.Color
}

type cardField struct {
	label string
	value string
}

func buildTitle(int) []hLine {
	return []hLine{{kind: hTitle}}
}

func homeFG(p palette, text string, c lipgloss.Color, bold bool) string {
	st := p.r.NewStyle().Inline(true).Foreground(c)
	if bold {
		st = st.Bold(true)
	}
	return st.Render(text)
}

func lerpTitle(n, i int) string {
	if n <= 1 {
		return titleStartHex
	}
	t := float64(i) / float64(n-1)
	r := uint8(math.Round(240 + (158-240)*t))
	g := uint8(math.Round(173 + (226-173)*t))
	b := uint8(math.Round(160 + (240-160)*t))
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func homeSystem(info platform.Info) string {
	if info.OS.SystemText == "" {
		return identityFallback
	}
	return info.OS.SystemText
}

func homeArch(info platform.Info) string {
	uname := info.Arch.UnameM
	if uname == "" {
		uname = identityFallback
	}
	fam := info.Arch.Family
	if fam == "" {
		fam = "unknown"
	}
	return fmt.Sprintf("%s（GOARCH=%s, uname-m=%s）", fam, info.Arch.GOARCH, uname)
}

func resolveIdentity(current func() (string, error), hostname func() (string, error)) (userName, host string) {
	return cleanIdentity(callIdentity(current)), cleanIdentity(callIdentity(hostname))
}

func callIdentity(fn func() (string, error)) string {
	if fn == nil {
		return ""
	}
	s, err := fn()
	if err != nil {
		return ""
	}
	return s
}

func cleanIdentity(s string) string {
	// Trim would hide a trailing newline. Reject the whole value first.
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return identityFallback
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return identityFallback
	}
	return s
}

// pickIP returns the first usable non-loopback IPv4 address.
func pickIP(addrs []net.Addr) string {
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipn.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		return ip.String()
	}
	return identityFallback
}

// parseResolv returns the nameserver entries of a resolv.conf body, comma separated.
func parseResolv(data string) string {
	var ns []string
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "nameserver" {
			ns = append(ns, f[1])
		}
	}
	if len(ns) == 0 {
		return identityFallback
	}
	return cleanIdentity(strings.Join(ns, ", "))
}

func readIdentity() identityMsg {
	name, host := resolveIdentity(func() (string, error) {
		cu, err := user.Current()
		if err != nil {
			return "", err
		}
		return cu.Username, nil
	}, os.Hostname)
	ip := identityFallback
	if addrs, err := net.InterfaceAddrs(); err == nil {
		ip = pickIP(addrs)
	}
	dns := identityFallback
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		dns = parseResolv(string(data))
	}
	return identityMsg{User: name, Host: host, IP: ip, DNS: dns}
}

func shownID(s string) string {
	if s == "" {
		return identityFallback
	}
	return s
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func padWidth(s string, w int) string {
	n := w - ansi.StringWidth(s)
	if n < 0 {
		n = 0
	}
	return s + spaces(n)
}

func fitWidth(s string, w int) string {
	if w < 0 {
		w = 0
	}
	return padWidth(ansi.Truncate(s, w, "…"), w)
}

func renderHomeBody(m model, stageH int, styled bool, p palette) []string {
	if stageH <= 0 {
		return nil
	}
	stack := buildStack(m, styled)
	if styled && len(stack) > stageH {
		stack = buildStack(m, false)
	}
	if len(stack) > stageH {
		stack = clipHome(stack, stageH)
	} else if top := (stageH - len(stack)) / 2; top > 0 {
		pad := make([]hLine, top)
		for i := range pad {
			pad[i] = hLine{kind: hBlank}
		}
		stack = append(pad, stack...)
	}
	out := make([]string, len(stack))
	for i, ln := range stack {
		out[i] = paintHLine(p, ln, m.width, styled)
	}
	return out
}

func clipHome(lines []hLine, stageH int) []hLine {
	clipped := clipLines(make([]string, len(lines)), stageH)
	if len(clipped) == 0 {
		return nil
	}
	keep := len(clipped) - 1
	if keep > len(lines) {
		keep = len(lines)
	}
	if keep < 0 {
		keep = 0
	}
	out := append([]hLine(nil), lines[:keep]...)
	return append(out, hLine{kind: hNote, text: clipped[len(clipped)-1]})
}

func buildStack(m model, big bool) []hLine {
	if m.phase == phaseUnavailable {
		return buildUnavailable(m, big)
	}
	return buildHome(m, big)
}

func labelColor(i, n int) lipgloss.Color {
	return lipgloss.Color(lerpTitle(n, i))
}

func buildHome(m model, big bool) []hLine {
	w := m.width
	if w < 5 {
		return append(buildTitle(w),
			hLine{kind: hBlank},
			menuLine(itemDownload, hintDownload, m.homeIndex == 0, w, 0),
			menuLine(itemUninstall, hintUninstall, m.homeIndex == 1, w, 0),
		)
	}
	host := shownID(m.homeHost)
	fields := []cardField{
		{"使用者", shownID(m.homeUser)},
		{"系統", homeSystem(m.info)},
		{"架構", homeArch(m.info)},
		{"IP", shownID(m.homeIP)},
		{"DNS", shownID(m.homeDNS)},
	}
	b := w - 4
	natural := ansi.StringWidth(host)
	for _, f := range fields {
		if rw := cardLabelW + 2 + ansi.StringWidth(f.value); rw > natural {
			natural = rw
		}
	}
	content := natural
	if content > b {
		content = b
	}
	cardW := content + 4
	left := 0
	if cardW < w {
		left = (w - cardW) / 2
	}
	lines := append(titleLines(w, big), hLine{kind: hBlank})
	lines = append(lines, homeCard(content, host, fields, left)...)
	lines = append(lines, hLine{kind: hBlank})
	lines = append(lines,
		menuLine(itemDownload, hintDownload, m.homeIndex == 0, cardW, left),
		menuLine(itemUninstall, hintUninstall, m.homeIndex == 1, cardW, left),
	)
	return lines
}

func buildUnavailable(m model, big bool) []hLine {
	w := m.width
	if w < 5 {
		text := ""
		if w > 0 {
			text = ansi.Truncate(unavailSentence, w, "…")
		}
		return append(buildTitle(w), hLine{kind: hNote, text: text})
	}
	b := w - 4
	parts := []string{itemUninstall, unavailLead, unavailSentence}
	natural := 0
	for _, s := range parts {
		if sw := ansi.StringWidth(s); sw > natural {
			natural = sw
		}
	}
	content := natural
	if natural > b {
		content = b
	}
	top, sep, bottom := cardRules(content)
	card := []hLine{
		{kind: hRule, text: top},
		{kind: hHead, text: cardInner(fitWidth(itemUninstall, content))},
		{kind: hRule, text: sep},
		{kind: hCard, text: cardInner(fitWidth(unavailLead, content))},
		{kind: hCard, text: cardInner(fitWidth(unavailSentence, content))},
		{kind: hRule, text: bottom},
	}
	cardW := content + 4
	left := 0
	if cardW < w {
		left = (w - cardW) / 2
	}
	for i := range card {
		card[i].left = left
	}
	return append(append(titleLines(w, big), hLine{kind: hBlank}), card...)
}

// fieldRow lays out one card row: the label padded to cardLabelW, two spaces, then the value.
func fieldRow(label, value string, content int) string {
	return fitWidth(padWidth(label, cardLabelW)+"  "+value, content)
}

func homeCard(content int, host string, fields []cardField, left int) []hLine {
	top, sep, bottom := cardRules(content)
	out := []hLine{
		{kind: hRule, text: top, left: left},
		{kind: hHead, text: cardInner(fitWidth(host, content)), left: left},
		{kind: hRule, text: sep, left: left},
	}
	for i, f := range fields {
		row := fieldRow(f.label, f.value, content)
		lab := f.label
		if ansi.StringWidth(lab) > content {
			lab = ""
		}
		out = append(out, hLine{kind: hCard, text: cardInner(row), left: left, label: lab, color: labelColor(i, len(fields))})
	}
	out = append(out, hLine{kind: hRule, text: bottom, left: left})
	return out
}

func cardRules(content int) (top, sep, bottom string) {
	b := lipgloss.RoundedBorder()
	if content < 0 {
		content = 0
	}
	n := content + 2
	// RoundedBorder.Middle is ┼, not the horizontal stroke.
	line := strings.Repeat(b.Top, n)
	return b.TopLeft + line + b.TopRight,
		b.MiddleLeft + line + b.MiddleRight,
		b.BottomLeft + strings.Repeat(b.Bottom, n) + b.BottomRight
}

func cardInner(body string) string {
	b := lipgloss.RoundedBorder()
	return b.Left + " " + body + " " + b.Right
}

func menuLine(name, hint string, selected bool, cardW, left int) hLine {
	mark := "  "
	if selected {
		mark = "> "
	}
	ln := hLine{kind: hMenu, selected: selected, left: left}
	if cardW >= 30 {
		namePad := 15 - ansi.StringWidth(name)
		if namePad < 0 {
			namePad = 0
		}
		gap := cardW - 2 - 15 - 12
		if gap < 0 {
			gap = 0
		}
		ln.text = mark + name
		ln.mid = spaces(namePad + gap)
		ln.hint = hint
		return ln
	}
	shown := ""
	if cardW > 0 {
		shown = ansi.Truncate(mark+name, cardW, "…")
	}
	pad := cardW - ansi.StringWidth(shown)
	if pad < 0 {
		pad = 0
	}
	ln.text = shown
	ln.mid = spaces(pad)
	return ln
}

func paintHLine(p palette, ln hLine, width int, styled bool) string {
	switch ln.kind {
	case hBlank:
		return homeBlank(p, width, styled)
	case hTitle:
		if styled {
			return paintHomeTitle(p, width)
		}
		return plainAsk(homeTitleText, width)
	case hBig:
		if !styled {
			return plainPad(ln.left, ln.text, width)
		}
		return paintBigRow(p, ln, width)
	case hRule:
		return plainPad(ln.left, ln.text, width)
	case hCard, hHead:
		if !styled {
			return plainPad(ln.left, ln.text, width)
		}
		return paintCardLine(p, ln, width)
	case hMenu:
		if !styled {
			return plainPad(ln.left, ln.text+ln.mid+ln.hint, width)
		}
		return paintMenuLine(p, ln, width)
	case hNote:
		if !styled {
			return ln.text
		}
		if ln.text == "" || width <= 0 {
			return ""
		}
		return plainPad(0, homeFG(p, ln.text, textColor, false), width)
	default:
		return ""
	}
}

func homeBlank(_ palette, _ int, _ bool) string {
	return ""
}

func plainPad(left int, text string, width int) string {
	if width <= 0 {
		return ""
	}
	line := spaces(left) + text
	if extra := width - ansi.StringWidth(line); extra > 0 {
		line += spaces(extra)
	}
	return line
}

func titleRunes(visible string) (glyphs []rune, ell bool) {
	if visible == "…" {
		return []rune("…"), false
	}
	rs := []rune(visible)
	if visible != homeTitleText && len(rs) > 0 && rs[len(rs)-1] == '…' {
		return rs[:len(rs)-1], true
	}
	return rs, false
}

func paintHomeTitle(p palette, width int) string {
	if width <= 0 {
		return ""
	}
	visible, left := placeHeader(homeTitleText, width)
	glyphs, ell := titleRunes(visible)
	n := len(glyphs)
	var b strings.Builder
	for i, r := range glyphs {
		b.WriteString(homeFG(p, string(r), lipgloss.Color(lerpTitle(n, i)), true))
	}
	if ell {
		b.WriteString(homeFG(p, "…", lipgloss.Color(lerpTitle(n, len(glyphs)-1)), true))
	}
	return plainPad(left, b.String(), width)
}

func paintCardLine(p palette, ln hLine, width int) string {
	if width <= 0 {
		return ""
	}
	rs := []rune(ln.text)
	core := ln.text
	if len(rs) >= 4 {
		body := string(rs[2 : len(rs)-2])
		var mid string
		switch {
		case ln.kind == hHead:
			mid = homeFG(p, body, headColor, true)
		case ln.label != "" && strings.HasPrefix(body, ln.label):
			mid = homeFG(p, ln.label, ln.color, true) + homeFG(p, body[len(ln.label):], textColor, false)
		default:
			mid = homeFG(p, body, textColor, false)
		}
		core = string(rs[:2]) + mid + string(rs[len(rs)-2:])
	}
	return plainPad(ln.left, core, width)
}

func paintMenuLine(p palette, ln hLine, width int) string {
	if width <= 0 {
		return ""
	}
	var core strings.Builder
	if ln.selected {
		if ln.text != "" {
			core.WriteString(p.ink(ln.text, inkColor, selectColor, true))
		}
		if ln.mid != "" {
			core.WriteString(p.gap(ln.mid, selectColor))
		}
		if ln.hint != "" {
			core.WriteString(p.ink(ln.hint, inkColor, selectColor, false))
		}
	} else {
		if ln.text != "" {
			core.WriteString(homeFG(p, ln.text, textColor, false))
		}
		core.WriteString(ln.mid)
		if ln.hint != "" {
			core.WriteString(homeFG(p, ln.hint, textColor, false))
		}
	}
	return plainPad(ln.left, core.String(), width)
}
