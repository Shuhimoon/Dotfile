package tui

import (
	"context"
	"os"
	"os/signal"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

const (
	uninstallNote  = "只列出已安裝的工具；基礎類（git、curl、vim…）不可移除"
	uninstallEmpty = "沒有可移除的已安裝工具"
	pythonConfirm  = "仍要移除 python 嗎？按 y 確定，n 或 esc 返回。"
)

// removableIDs are the installed, removable tools, in catalog order.
func (m model) removableIDs() []catalog.ID {
	var out []catalog.ID
	for _, id := range m.flat {
		if m.installed[id] && uninstall.Removable(id) && uninstall.Supported(id, m.info.Manager) {
			out = append(out, id)
		}
	}
	return out
}

func (m model) uninstallRows() (rows []catalogRow, cursorAt int) {
	ids := m.removableIDs()
	show := map[catalog.ID]bool{}
	for _, id := range ids {
		show[id] = true
	}
	cur := catalog.ID("")
	if m.ucursor >= 0 && m.ucursor < len(ids) {
		cur = ids[m.ucursor]
	}
	for ci, c := range m.cats {
		header := catalogRow{Kind: rowHeader, Title: c.Title, Cat: ci}
		added := false
		for _, it := range c.Items {
			if !show[it.ID] {
				continue
			}
			if !added {
				rows = append(rows, header)
				added = true
			}
			it.Default = false
			if _, warn := uninstall.NeedsWarning(it.ID); warn {
				it.Description = "移除前會先警告"
			}
			row := catalogRow{Kind: rowItem, Item: it, Selected: m.uselected[it.ID], Cat: ci}
			if it.ID == cur {
				row.Cursor = true
				cursorAt = len(rows)
			}
			rows = append(rows, row)
		}
	}
	return rows, cursorAt
}

func (m model) uninstallSelected() []catalog.ID {
	var out []catalog.ID
	for _, id := range m.removableIDs() {
		if m.uselected[id] {
			out = append(out, id)
		}
	}
	return out
}

func (m model) enterUninstall() (tea.Model, tea.Cmd) {
	m.phase = phaseUninstall
	m.mode = modeUninstall
	m.ucursor = 0
	m.uselected = map[catalog.ID]bool{}
	m.status = uninstallNote
	if len(m.removableIDs()) == 0 {
		m.status = uninstallEmpty
	}
	return m, nil
}

func (m model) uninstallKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	ids := m.removableIDs()
	if m.ucursor >= len(ids) {
		m.ucursor = len(ids) - 1
	}
	if m.ucursor < 0 {
		m.ucursor = 0
	}
	switch {
	case key.Matches(k, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(k, m.keys.Back):
		m.phase = phaseHome
		m.mode = modeInstall
		m.status = ""
		return m, nil
	case key.Matches(k, m.keys.Up):
		if m.ucursor > 0 {
			m.ucursor--
		}
	case key.Matches(k, m.keys.Down):
		if m.ucursor < len(ids)-1 {
			m.ucursor++
		}
	case key.Matches(k, m.keys.Top):
		m.ucursor = 0
	case key.Matches(k, m.keys.Bottom):
		if len(ids) > 0 {
			m.ucursor = len(ids) - 1
		}
	case key.Matches(k, m.keys.Toggle):
		if len(ids) > 0 {
			id := ids[m.ucursor]
			m.uselected[id] = !m.uselected[id]
		}
	case key.Matches(k, m.keys.CatAll):
		if len(ids) > 0 {
			cat := m.categoryOf(ids[m.ucursor])
			var in []catalog.ID
			for _, id := range ids {
				if m.categoryOf(id) == cat {
					in = append(in, id)
				}
			}
			m.toggleRemove(in)
		}
	case key.Matches(k, m.keys.All):
		m.toggleRemove(ids)
	case key.Matches(k, m.keys.Install):
		sel := m.uninstallSelected()
		if len(sel) == 0 {
			m.status = "尚未勾選任何項目"
			return m, nil
		}
		for _, id := range sel {
			if _, warn := uninstall.NeedsWarning(id); warn {
				m.phase = phasePythonWarn
				m.status = ""
				return m, nil
			}
		}
		return m.enterUninstallConfirm()
	}
	return m, nil
}

func (m *model) toggleRemove(ids []catalog.ID) {
	allOn := len(ids) > 0
	for _, id := range ids {
		if !m.uselected[id] {
			allOn = false
			break
		}
	}
	for _, id := range ids {
		m.uselected[id] = !allOn
	}
}

func (m model) pythonWarnKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "y":
		return m.enterUninstallConfirm()
	case "n", "esc":
		m.phase = phaseUninstall
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) exists() func(string) bool {
	if m.deps.Exists != nil {
		return m.deps.Exists
	}
	return statExists
}

func (m model) enterUninstallConfirm() (tea.Model, tea.Cmd) {
	steps, err := uninstall.BuildSteps(m.uninstallSelected(), m.info.Manager, os.Geteuid(), uninstall.Probe{
		Home:   m.home(),
		GoRoot: m.deps.Runner.GoRoot,
		Exists: m.exists(),
	})
	if err != nil {
		m.phase = phaseUninstall
		m.status = err.Error()
		return m, nil
	}
	if len(steps) == 0 {
		m.phase = phaseUninstall
		m.status = "沒有要執行的步驟"
		return m, nil
	}
	m.usteps = steps
	m.steps = make([]install.Step, len(steps))
	for i, st := range steps {
		m.steps[i] = install.Step{ItemID: st.ItemID, Title: st.Title}
	}
	m.mode = modeUninstall
	m.dryRun = false
	m.phase = phaseConfirm
	m.status = ""
	return m, nil
}

func (m model) startUninstall() (tea.Model, tea.Cmd) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	m.cancel = stop
	m.phase = phaseInstalling
	m.results = nil
	m.done = nil
	m.curTitle = ""
	m.running = false
	m.status = ""
	m.runID++
	ch := make(chan bridgeMsg)
	m.events = ch
	runID := m.runID
	src := m.deps.Runner
	r := uninstall.Runner{
		Exec:   teaExec{ch: ch},
		Home:   src.Home,
		XDG:    src.XDG,
		Repo:   src.Repo,
		Bashrc: src.Bashrc,
		Zshrc:  src.Zshrc,
		Now:    src.Now,
	}
	if r.Home == "" {
		r.Home = m.home()
	}
	r.Progress = func(title, status string) {
		ch <- bridgeMsg{kind: kindProgress, title: title, status: status}
	}
	steps := append([]uninstall.Step(nil), m.usteps...)
	go func() {
		results, code := r.Run(ctx, steps)
		ch <- bridgeMsg{kind: kindDone, results: results, code: code}
	}()
	return m, listen(ch, runID)
}
