package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

// The progress page (phaseInstalling, then phaseReport) stays inside the TUI
// for the whole run: per-step status, "3/7" counter, and a tail of the live
// command output streamed through bridgeMsg.

const (
	stRunning  = "running"
	stCanceled = "canceled"
	// outputCap bounds the kept output tail.
	outputCap = 500
	// sudoRefresh keeps the sudo credential cache warm during a long run.
	sudoRefresh = 60 * time.Second
)

// launch starts the prepared run (m.mode, m.steps / m.usteps) in a goroutine.
// Commands go through streamExec unless Deps.Runner.Exec is set (tests).
func (m model) launch(mode sudoMode, pw []byte) (tea.Model, tea.Cmd) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	m.cancel = stop
	m.phase = phaseInstalling
	m.results = nil
	m.exitCode = 0
	m.curTitle = ""
	m.output = nil
	m.outCR = false
	m.canceled = false
	m.stepState = make([]string, len(m.steps))
	m.stepRun = -1
	m.stepNext = 0
	m.status = ""
	m.runID++
	ch := make(chan bridgeMsg, 64)
	m.events = ch
	runID := m.runID
	ex := streamExec{ch: ch, mode: mode, pw: pw}
	progress := func(title, status string) {
		ch <- bridgeMsg{kind: kindProgress, title: title, status: status}
	}
	var work func() ([]install.ItemResult, int)
	needSudo := false
	if m.mode == modeUninstall {
		needSudo = uninstallNeedsSudo(m.usteps)
		src := m.deps.Runner
		var uex install.Executor = ex
		if src.Exec != nil {
			uex = src.Exec
		}
		r := uninstall.Runner{
			Exec: uex, Home: src.Home, XDG: src.XDG, Repo: src.Repo,
			Bashrc: src.Bashrc, Zshrc: src.Zshrc, Now: src.Now, Progress: progress,
		}
		if r.Home == "" {
			r.Home = m.home()
		}
		steps := append([]uninstall.Step(nil), m.usteps...)
		work = func() ([]install.ItemResult, int) { return r.Run(ctx, steps) }
	} else {
		needSudo = installNeedsSudo(m.steps)
		r := m.deps.Runner
		r.Force = m.force || m.reinstalling()
		if r.Exec == nil {
			r.Exec = ex
		}
		r.EUID = os.Geteuid()
		r.Manager = m.info.Manager
		if r.Home == "" {
			r.Home = m.home()
		}
		if m.deps.Log != nil {
			mgr := m.info.Manager
			logFn := m.deps.Log
			r.Log = func(results []install.ItemResult) error { return logFn(mgr, results) }
		}
		r.Progress = progress
		steps := append([]install.Step(nil), m.steps...)
		work = func() ([]install.ItemResult, int) { return r.RunSteps(ctx, steps) }
	}
	s := m.sudo()
	go func() {
		finished := make(chan struct{})
		if needSudo && mode == sudoCached {
			go keepSudo(ctx, s, finished)
		}
		results, code := work()
		close(finished)
		clear(pw)
		ch <- bridgeMsg{kind: kindDone, results: results, code: code}
	}()
	return m, listen(ch, runID)
}

func keepSudo(ctx context.Context, s sudoer, finished <-chan struct{}) {
	t := time.NewTicker(sudoRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finished:
			return
		case <-t.C:
			s.Refresh(ctx)
		}
	}
}

// reinstalling: an installed tool picked in the list means "download again".
func (m model) reinstalling() bool {
	for _, id := range m.selectedIDs() {
		if m.installed[id] {
			return true
		}
	}
	return false
}

func (m model) cancelRun() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	m.canceled = true
	return m, nil
}

func (m model) onBridge(msg bridgeMsg) (tea.Model, tea.Cmd) {
	if msg.runID != m.runID || m.phase != phaseInstalling {
		return m, nil
	}
	switch msg.kind {
	case kindProgress:
		m.applyProgress(msg.title, msg.status)
	case kindOutput:
		m.appendOutput(msg.line, msg.cr)
	case kindDone:
		m.finishRun(msg.results, msg.code)
		return m, m.redetect()
	}
	return m, listen(m.events, m.runID)
}

func (m *model) applyProgress(title, status string) {
	if len(m.stepState) != len(m.steps) {
		m.stepState = make([]string, len(m.steps))
		m.stepRun = -1
	}
	if status == "" {
		m.curTitle = title
		if m.stepRun >= 0 {
			return // a fallback inside the running step
		}
		if i := m.findStep(title); i >= 0 {
			m.stepState[i] = stRunning
			m.stepRun = i
			m.stepNext = i + 1
		}
		return
	}
	i := m.stepRun
	if i < 0 {
		i = m.findStep(title)
		if i >= 0 {
			m.stepNext = i + 1
		}
	}
	if i >= 0 {
		m.stepState[i] = status
	}
	m.stepRun = -1
}

func (m model) findStep(title string) int {
	for j := m.stepNext; j < len(m.steps); j++ {
		if m.steps[j].Title == title {
			return j
		}
	}
	if m.stepNext < len(m.steps) {
		return m.stepNext
	}
	return -1
}

func (m *model) appendOutput(line string, cr bool) {
	if m.outCR && len(m.output) > 0 {
		m.output[len(m.output)-1] = line
	} else {
		m.output = append(m.output, line)
	}
	m.outCR = cr
	if len(m.output) > outputCap {
		m.output = append([]string(nil), m.output[len(m.output)-outputCap:]...)
	}
}

func (m *model) finishRun(results []install.ItemResult, code int) {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.stepRun >= 0 && m.stepRun < len(m.stepState) {
		if m.canceled {
			m.stepState[m.stepRun] = stCanceled
		} else {
			m.stepState[m.stepRun] = "failed"
		}
		m.stepRun = -1
	}
	m.results = results
	m.exitCode = code
	m.phase = phaseReport
}

// redetect re-probes what is installed, for the list and the home page.
func (m model) redetect() tea.Cmd {
	detect := m.deps.Detect
	if detect == nil {
		detect = detectInstalled
	}
	manager, runID := m.info.Manager, m.runID
	return func() tea.Msg {
		return installedMsg{runID: runID, installed: detect(manager)}
	}
}

// goHome leaves the report: any key lands on the home page, re-detected.
// backToList leaves the d-key plan for the list it came from, ticks kept.
func (m model) backToList() (tea.Model, tea.Cmd) {
	m.dryRun = false
	m.plan = nil
	m.warnings = nil
	m.status = ""
	if m.mode == modeUninstall {
		m.phase = phaseUninstall
	} else {
		m.phase = phaseBrowsing
	}
	return m, nil
}

func (m model) goHome() (tea.Model, tea.Cmd) {
	m.phase = phaseHome
	m.mode = modeInstall
	m.status = ""
	m.plan = nil
	m.dryRun = false
	m.warnings = nil
	return m, m.redetect()
}

type tone int

const (
	toneText tone = iota
	toneHead
	toneRun
	toneOK
	toneFail
	toneMuted
	toneWarn
	toneOut
	tonePlain // white: the d-key plan
)

type pline struct {
	text string
	tone tone
}

func statusLabel(s string) string {
	switch s {
	case "":
		return "等待"
	case stRunning:
		return "執行中"
	case "ok", "shadowed", uninstall.StatusRemoved:
		return "完成"
	case "satisfied":
		return "已安裝"
	case "skipped":
		return "略過"
	case "failed":
		return "失敗"
	case stCanceled:
		return "已取消"
	case "pending-user":
		return "待處理"
	default:
		return s
	}
}

func statusTone(s string) tone {
	switch s {
	case "":
		return toneMuted
	case stRunning:
		return toneRun
	case "ok", "shadowed", "satisfied", uninstall.StatusRemoved:
		return toneOK
	case "failed", stCanceled, "pending-user":
		return toneFail
	default:
		return toneMuted
	}
}

func (m model) stepCounter() (cur, total int) {
	total = len(m.steps)
	for _, s := range m.stepState {
		if s != "" && s != stRunning {
			cur++
		}
	}
	if m.stepRun >= 0 {
		cur = m.stepRun + 1
	}
	return cur, total
}

func (m model) progressHead() pline {
	verb := "安裝"
	if m.mode == modeUninstall {
		verb = "移除"
	}
	cur, total := m.stepCounter()
	if m.phase == phaseInstalling {
		if m.canceled {
			return pline{"取消中…", toneWarn}
		}
		text := fmt.Sprintf("%s中  %d/%d", verb, cur, total)
		if m.curTitle != "" {
			text += "  " + m.curTitle
		}
		return pline{text, toneHead}
	}
	if m.dryRun {
		return pline{"只顯示計畫，沒有執行", tonePlain}
	}
	if m.canceled {
		return pline{fmt.Sprintf("%s已取消（%d/%d）", verb, cur, total), toneWarn}
	}
	ok, bad := 0, 0
	for _, r := range m.results {
		switch statusTone(r.Status) {
		case toneOK:
			ok++
		case toneFail:
			bad++
		}
	}
	if m.exitCode == 2 && len(m.results) == 0 {
		return pline{verb + "失敗：更新套件清單失敗", toneFail}
	}
	text := fmt.Sprintf("%s完成：成功 %d", verb, ok)
	if bad > 0 {
		text += fmt.Sprintf("、失敗 %d", bad)
		return pline{text, toneFail}
	}
	return pline{text, toneOK}
}

func (m model) stepLines() []pline {
	out := make([]pline, 0, len(m.steps))
	for i, st := range m.steps {
		s := ""
		if i < len(m.stepState) {
			s = m.stepState[i]
		}
		out = append(out, pline{padWidth(statusLabel(s), 6) + "  " + st.Title, statusTone(s)})
	}
	return out
}

func (m model) resultLines() []pline {
	out := make([]pline, 0, len(m.results))
	for _, r := range m.results {
		out = append(out, pline{install.FormatResult(r), statusTone(r.Status)})
	}
	return out
}

// progressLines lays the page out in exactly bodyH lines or fewer:
// head, warnings, steps (or results) windowed, then the output tail.
func progressLines(m model, bodyH int) []pline {
	if bodyH <= 0 {
		return nil
	}
	lines := []pline{m.progressHead()}
	for _, w := range m.warnings {
		// Warnings wrap instead of being cut: they are the only place the
		// python risk is shown now that nothing asks for y.
		for _, part := range strings.Split(ansi.Hardwrap(w, max(m.width, 1), true), "\n") {
			lines = append(lines, pline{part, toneWarn})
		}
	}
	if len(lines) >= bodyH {
		return lines[:bodyH]
	}
	list := m.stepLines()
	focus := m.stepRun
	if focus < 0 {
		focus = m.stepNext
	}
	if m.phase == phaseReport && len(m.results) > 0 {
		list = m.resultLines()
		focus = 0
	}
	if m.phase == phaseReport && m.dryRun {
		list = make([]pline, 0, len(m.plan))
		for _, s := range m.plan {
			list = append(list, pline{s, tonePlain})
		}
		focus = 0
	}
	room := bodyH - len(lines)
	outN := 0
	if len(m.output) > 0 && room >= 4 {
		// Keep at least half the room for the list, the rest for the tail.
		listN := len(list)
		if listN > room/2 {
			listN = room / 2
		}
		outN = room - listN - 1
		if outN > len(m.output) {
			outN = len(m.output)
		}
	}
	listRoom := room
	if outN > 0 {
		listRoom = room - outN - 1
	}
	start, end := window(len(list), focus, 0, listRoom)
	lines = append(lines, list[start:end]...)
	if hidden := len(list) - end; hidden > 0 && end > start {
		lines[len(lines)-1] = pline{fmt.Sprintf("…還有 %d 項", hidden+1), toneMuted}
	}
	if outN > 0 {
		lines = append(lines, pline{"輸出", toneMuted})
		for _, o := range m.output[len(m.output)-outN:] {
			lines = append(lines, pline{"  " + o, toneOut})
		}
	}
	return lines
}

func plainProgress(m model, bodyH int) []string {
	pl := progressLines(m, bodyH)
	out := make([]string, len(pl))
	for i, l := range pl {
		out[i] = fitLine(l.text, m.width)
	}
	return out
}

// fitLine cuts a line to the window. A cut line keeps one spare column: some
// terminals (tmux) draw the "…" two columns wide, and a wrapped line would push
// the title bar off the top of the frame.
func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > width {
		if width > 1 {
			width--
		}
		return ansi.Truncate(s, width, "…")
	}
	return s
}

func paintProgress(p palette, m model, bodyH int) []string {
	pl := progressLines(m, bodyH)
	out := make([]string, len(pl))
	for i, l := range pl {
		fg, bold := p.cream, false
		switch l.tone {
		case toneHead:
			fg, bold = p.cream, true
		case toneRun:
			fg = selectColor
		case toneOK:
			fg = p.key
		case toneFail:
			fg = failColor
		case toneMuted:
			fg = p.muted
		case toneWarn:
			fg = p.gold
		case toneOut:
			fg = p.sand
		case tonePlain:
			fg = planColor
		}
		text := fitLine(strings.TrimRight(l.text, " "), m.width)
		out[i] = p.padTo(p.ink(text, fg, p.bark, bold), m.width, p.bark)
	}
	return out
}
