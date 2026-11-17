package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

const installedLabel = "(已安裝)"

func FormatItem(it catalog.Item, selected bool) string {
	mark := " "
	if it.Default {
		mark = "*"
	}
	box := "[ ]"
	if selected {
		box = "[x]"
	}
	return mark + " " + box + " " + it.Name
}

func FormatHeader(info platform.Info) string {
	uname := info.Arch.UnameM
	if uname == "" {
		uname = "未知"
	}
	pmName := info.Manager
	if pmName == "" {
		pmName = "未知"
	}
	fam := info.Arch.Family
	if fam == "" {
		fam = "unknown"
	}
	return fmt.Sprintf("系統: %s    架構: %s（GOARCH=%s, uname-m=%s）    套件: %s",
		info.OS.SystemText, fam, info.Arch.GOARCH, uname, pmName)
}

type rowKind int

const (
	rowHeader rowKind = iota
	rowItem
)

type catalogRow struct {
	Kind     rowKind
	Title    string
	Item     catalog.Item
	Selected bool
	Cursor   bool
	// Cat is the category index, used for the header gradient.
	Cat int
}

func (m model) View() string {
	return render(m, m.deps.Styled)
}

func render(m model, styled bool) string {
	if !styled {
		return renderPlain(m)
	}
	return renderStyled(m, lipgloss.DefaultRenderer())
}

func renderStyled(m model, r *lipgloss.Renderer) string {
	if m.height <= 0 {
		return ""
	}
	p := earthStyles(r)
	headerN, statusN, bodyH, footerN := frameSize(m, true)
	lines := make([]string, 0, m.height)
	if headerN == 1 {
		lines = append(lines, paintTitle(p, FormatHeader(m.info), m.width))
	}
	var body []string
	var fill string
	if !catalogChrome(m.phase) {
		fill = homeBlank(p, m.width, true)
		body = renderHomeBody(m, bodyH, true, p)
	} else {
		body = styledBody(p, m, bodyH)
		fill = p.blank(m.width)
		if showsStatus(m.phase) {
			fill = catalogBlank(p, m.width)
		}
	}
	if m.askName != "" && bodyH > 0 {
		body = placeAsk(body, bodyH, fill, paintHeader(p, m.askText(), m.width, false))
	}
	lines = append(lines, body...)
	for i := len(body); i < bodyH; i++ {
		lines = append(lines, fill)
	}
	lines = append(lines, styledStatus(p, m, statusN)...)
	if !catalogChrome(m.phase) {
		lines = append(lines, paintHomeFooter(p, m, footerN)...)
	} else {
		lines = append(lines, paintFooter(p, m, footerN)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderPlain(m model) string {
	if m.height <= 0 {
		return ""
	}
	headerN, statusN, bodyH, footerN := frameSize(m, false)
	lines := make([]string, 0, m.height)
	if headerN == 1 {
		lines = append(lines, plainHeader(m))
	}
	var body []string
	if !catalogChrome(m.phase) {
		body = renderHomeBody(m, bodyH, false, palette{})
	} else {
		body = plainBody(m, bodyH)
	}
	if m.askName != "" && bodyH > 0 {
		body = placeAsk(body, bodyH, "", plainAsk(m.askText(), m.width))
	}
	lines = append(lines, body...)
	for i := len(body); i < bodyH; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, statusLines(m, statusN)...)
	if footerN > 0 {
		lines = append(lines, plainHelp(m))
	}
	return strings.Join(lines, "\n") + "\n"
}

func frameSize(m model, styled bool) (headerN, statusN, bodyH, footerN int) {
	footerN = footerLineCount(m, styled)
	if footerN > m.height {
		footerN = m.height
	}
	if footerN < 0 {
		footerN = 0
	}
	if catalogChrome(m.phase) && m.height-footerN >= 1 {
		headerN = 1
	}
	if showsStatus(m.phase) && m.height-footerN-headerN >= 1 {
		statusN = 1
		if m.status != "" && m.height-footerN-headerN-statusN >= 1 {
			statusN = 2
		}
	}
	bodyH = m.height - headerN - footerN - statusN
	if bodyH < 0 {
		bodyH = 0
	}
	return headerN, statusN, bodyH, footerN
}

func catalogChrome(ph phase) bool {
	switch ph {
	case phaseDetecting, phaseHome:
		return false
	default:
		return true
	}
}

func showsStatus(ph phase) bool {
	return ph == phaseBrowsing || ph == phaseFailed || ph == phaseUninstall
}

func (m model) catalogRows() (rows []catalogRow, cursorAt int) {
	if m.phase == phaseUninstall {
		return m.uninstallRows()
	}
	cur := catalog.ID("")
	if m.cursor >= 0 && m.cursor < len(m.flat) {
		cur = m.flat[m.cursor]
	}
	for ci, c := range m.cats {
		rows = append(rows, catalogRow{Kind: rowHeader, Title: c.Title, Cat: ci})
		for _, it := range c.Items {
			row := catalogRow{
				Kind:     rowItem,
				Item:     it,
				Selected: m.selected[it.ID],
				Cat:      ci,
			}
			if it.ID == cur {
				row.Cursor = true
				cursorAt = len(rows)
			}
			rows = append(rows, row)
		}
	}
	return rows, cursorAt
}

func (m model) catalogLines() (lines []string, cursorAt int) {
	rows, cursorAt := m.catalogRows()
	lines = make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, formatRow(m, row))
	}
	return lines, cursorAt
}

func horizontalInset(width int) int {
	if width >= 5 {
		return 2
	}
	return 0
}

func catalogNameWidth() int {
	maxW := 0
	for _, c := range catalog.All() {
		for _, it := range c.Items {
			if w := ansi.StringWidth(it.Name); w > maxW {
				maxW = w
			}
		}
	}
	return maxW
}

func placeHeader(text string, width int) (visible string, left int) {
	if width <= 0 {
		return "", 0
	}
	visible = ansi.Truncate(text, width, "…")
	w := ansi.StringWidth(visible)
	if w >= width {
		return visible, 0
	}
	return visible, (width - w) / 2
}

func plainHeader(m model) string {
	return plainAsk(FormatHeader(m.info), m.width)
}

func (m model) askText() string {
	if m.askName == "" {
		return ""
	}
	if m.askTold {
		return "請加上 --force 後重新執行"
	}
	return m.askName + " 已安裝。要強制重新下載嗎？"
}

func plainAsk(text string, width int) string {
	if width <= 0 {
		return ""
	}
	visible, left := placeHeader(text, width)
	line := strings.Repeat(" ", left) + visible
	if w := ansi.StringWidth(line); width > w {
		line += strings.Repeat(" ", width-w)
	}
	return line
}

func placeAsk(body []string, bodyH int, fill, line string) []string {
	out := make([]string, bodyH)
	for i := 0; i < bodyH; i++ {
		if i < len(body) {
			out[i] = body[i]
		} else {
			out[i] = fill
		}
	}
	out[(bodyH-1)/2] = line
	return out
}

func formatRow(m model, row catalogRow) string {
	inset := horizontalInset(m.width)
	inner := m.width - 2*inset
	if inner < 0 {
		inner = 0
	}
	installed := m.rowInstalled(row)
	var line string
	if row.Kind == rowHeader {
		line = row.Title
	} else {
		line = itemLine(m, row)
	}
	if inner <= 0 {
		return ""
	}
	if installed {
		return strings.Repeat(" ", inset) + line + strings.Repeat(" ", inset)
	}
	if ansi.StringWidth(line) > inner {
		line = ansi.Truncate(line, inner, "…")
		if w := ansi.StringWidth(line); w < inner {
			line += strings.Repeat(" ", inner-w)
		}
	}
	return strings.Repeat(" ", inset) + line
}

func itemLeft(width int, row catalogRow) string {
	var b strings.Builder
	if row.Cursor {
		b.WriteString("> ")
	} else {
		b.WriteString("  ")
	}
	if row.Item.Default {
		b.WriteByte('*')
	} else {
		b.WriteByte(' ')
	}
	b.WriteByte(' ')
	if row.Selected {
		b.WriteString("[x]")
	} else {
		b.WriteString("[ ]")
	}
	b.WriteByte(' ')
	name := row.Item.Name
	b.WriteString(name)
	if pad := catalogNameWidth() - ansi.StringWidth(name); pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	if width >= 100 && row.Item.Description != "" {
		b.WriteString("  ")
		b.WriteString(row.Item.Description)
	}
	return b.String()
}

func placeInstalled(left string, inner int) (visible, gap, mark string) {
	const full = installedLabel // display width 8
	if inner <= 0 {
		return "", "", ""
	}
	if inner < 8 {
		mark = ansi.Truncate(full, inner, "…")
		pad := inner - ansi.StringWidth(mark)
		if pad < 0 {
			pad = 0
		}
		return strings.Repeat(" ", pad), "", mark
	}
	budget := inner - 8
	visible = left
	if ansi.StringWidth(visible) > budget {
		visible = ansi.Truncate(visible, budget, "…")
	}
	gapN := budget - ansi.StringWidth(visible)
	if gapN < 0 {
		gapN = 0
	}
	return visible, strings.Repeat(" ", gapN), full
}

func itemLine(m model, row catalogRow) string {
	left := itemLeft(m.width, row)
	if !m.rowInstalled(row) {
		return left
	}
	inset := horizontalInset(m.width)
	inner := m.width - 2*inset
	visible, gap, mark := placeInstalled(left, inner)
	return visible + gap + mark
}

func plainBody(m model, bodyH int) []string {
	if bodyH <= 0 {
		return nil
	}
	if showsStatus(m.phase) {
		lines, cursorAt := m.catalogLines()
		start, end := window(len(lines), cursorAt, m.scroll, bodyH)
		return lines[start:end]
	}
	return clipLines(phaseBody(m), bodyH)
}

func styledBody(p palette, m model, bodyH int) []string {
	if bodyH <= 0 {
		return nil
	}
	if showsStatus(m.phase) {
		rows, cursorAt := m.catalogRows()
		start, end := window(len(rows), cursorAt, m.scroll, bodyH)
		showDesc := m.width >= 100
		out := make([]string, 0, end-start)
		for _, row := range rows[start:end] {
			if row.Kind == rowHeader {
				out = append(out, paintCatalogHeader(p, row.Title, m.width, catColor(row.Cat, len(m.cats))))
				continue
			}
			out = append(out, paintItem(p, row, m.width, showDesc, m.rowInstalled(row)))
		}
		return out
	}
	raw := clipLines(phaseBody(m), bodyH)
	out := make([]string, len(raw))
	for i, line := range raw {
		out[i] = paintBodyLine(p, line, m.width)
	}
	return out
}

func phaseBody(m model) []string {
	switch m.phase {
	case phasePythonWarn:
		return []string{uninstall.PythonWarning, pythonConfirm}
	case phaseConfirm:
		var lines []string
		if m.mode == modeUninstall {
			lines = append(lines, "將移除下列項目，連同它的設定檔與 ~/.bashrc、~/.zshrc 設定區塊")
		}
		if m.dryRun {
			lines = append(lines, "只顯示計畫，不會執行")
		}
		for _, st := range m.steps {
			lines = append(lines, st.Title)
		}
		return lines
	case phaseInstalling:
		lines := []string{"目前  " + m.curTitle, "已完成"}
		for _, d := range m.done {
			lines = append(lines, fmt.Sprintf("  %s  %s", d.Title, d.Status))
		}
		lines = append(lines, "待執行")
		for _, st := range m.pendingSteps() {
			lines = append(lines, "  "+st.Title)
		}
		return lines
	case phaseReport:
		lines := make([]string, 0, len(m.results)+1)
		for _, res := range m.results {
			lines = append(lines, install.FormatResult(res))
		}
		if m.status != "" {
			lines = append(lines, m.status)
		}
		return lines
	default:
		return nil
	}
}

func clipLines(lines []string, bodyH int) []string {
	if bodyH <= 0 {
		return nil
	}
	if len(lines) <= bodyH {
		return lines
	}
	keep := bodyH - 1
	if keep < 0 {
		keep = 0
	}
	n := len(lines) - keep
	out := make([]string, 0, keep+1)
	if keep > 0 {
		out = append(out, lines[:keep]...)
	}
	out = append(out, fmt.Sprintf("…還有 %d 行", n))
	return out
}

func selectedLine(m model) string {
	if m.phase == phaseUninstall {
		id := ""
		if ids := m.removableIDs(); m.ucursor >= 0 && m.ucursor < len(ids) {
			id = string(ids[m.ucursor])
		}
		return fmt.Sprintf("已勾選移除 %d    游標 %s", len(m.uninstallSelected()), id)
	}
	return fmt.Sprintf("已勾選 %d    游標 %s", m.countSelected(), m.cursorID())
}

func statusLines(m model, statusN int) []string {
	switch {
	case statusN <= 0:
		return nil
	case statusN == 1 && m.status != "":
		return []string{m.status}
	case statusN == 1:
		return []string{selectedLine(m)}
	default:
		return []string{selectedLine(m), m.status}
	}
}

func styledStatus(p palette, m model, statusN int) []string {
	raw := statusLines(m, statusN)
	out := make([]string, len(raw))
	for i, line := range raw {
		fg := p.muted
		if (statusN == 2 && i == 1) || (statusN == 1 && m.status != "") {
			fg = p.cream
		}
		out[i] = paintStatusLine(p, line, fg, m.width)
	}
	return out
}

// window is the catalog viewport. scroll is not written back; View must not change the model.
func window(n, cursorAt, scroll, height int) (start, end int) {
	if height <= 0 || n <= 0 {
		return 0, 0
	}
	if scroll > cursorAt {
		scroll = cursorAt
	}
	if cursorAt >= scroll+height {
		scroll = cursorAt - height + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	end = scroll + height
	if end > n {
		end = n
	}
	if scroll > n {
		scroll = n
	}
	if scroll > end {
		scroll = end
	}
	return scroll, end
}

func (m model) cursorID() string {
	if m.cursor < 0 || m.cursor >= len(m.flat) {
		return ""
	}
	return string(m.flat[m.cursor])
}

func (m model) pendingSteps() []install.Step {
	n := len(m.done)
	if m.running {
		n++
	}
	if n >= len(m.steps) {
		return nil
	}
	return m.steps[n:]
}

// rowInstalled is the purple "(已安裝)" style. The uninstall list only shows
// installed tools, so it uses the plain row style there.
func (m model) rowInstalled(row catalogRow) bool {
	return row.Kind == rowItem && m.phase != phaseUninstall && m.installed[row.Item.ID]
}
