package tui

import (
	"context"
	"os"
	"os/signal"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		return m, nil
	case detectDoneMsg:
		m.info = msg.Info
		if msg.Err != nil {
			m.phase = phaseFailed
			m.status = msg.Err.Error()
			m.exitCode = 2
			return m, nil
		}
		m.applyInstalled(msg.Installed)
		m.phase = phaseHome
		m.status = ""
		return m, nil
	case identityMsg:
		m.homeUser = msg.User
		m.homeHost = msg.Host
		m.homeIP = msg.IP
		m.homeDNS = msg.DNS
		return m, nil
	case installedMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		m.applyInstalled(msg.installed)
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	case bridgeMsg:
		return m.onBridge(msg)
	case listenAgainMsg:
		if msg.runID != m.runID || m.phase != phaseInstalling {
			return m, nil
		}
		return m, listen(m.events, m.runID)
	}
	return m, nil
}

func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.askName != "" {
		return m.onAskKey(k)
	}
	switch m.phase {
	case phaseDetecting:
		return m.homeKey(k, false)
	case phaseHome:
		return m.homeKey(k, true)
	case phaseUninstall:
		return m.uninstallKey(k)
	case phasePythonWarn:
		return m.pythonWarnKey(k)
	case phaseFailed:
		if k.String() == "q" {
			return m, tea.Quit
		}
		if k.String() == "enter" || k.String() == "d" || k.String() == "ctrl+c" {
			return m, nil
		}
		return m.browseKey(k, false)
	case phaseBrowsing:
		if key.Matches(k, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(k, m.keys.Back) {
			m.phase = phaseHome
			return m, nil
		}
		return m.browseKey(k, true)
	case phaseConfirm:
		switch k.String() {
		case "y":
			if m.mode == modeUninstall {
				return m.startUninstall()
			}
			return m.startInstall()
		case "n", "esc":
			m.phase = phaseBrowsing
			if m.mode == modeUninstall {
				m.phase = phaseUninstall
			}
			m.dryRun = false
			return m, nil
		}
	case phaseInstalling:
		if k.String() == "ctrl+c" {
			return m.cancelInstall()
		}
	case phaseReport:
		if k.String() == "q" {
			return m, tea.Quit
		}
		m.phase = phaseBrowsing
		if m.mode == modeUninstall {
			m.phase = phaseUninstall
			m.status = uninstallNote
		}
		m.dryRun = false
		return m, nil
	}
	return m, nil
}

func (m model) homeKey(k tea.KeyMsg, allowEnter bool) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, m.keys.Up):
		if m.homeIndex > 0 {
			m.homeIndex--
		}
	case key.Matches(k, m.keys.Down):
		if m.homeIndex < 1 {
			m.homeIndex++
		}
	case key.Matches(k, m.keys.Quit):
		return m, tea.Quit
	case allowEnter && k.String() == "enter":
		return m.activateHome()
	}
	return m, nil
}

func (m model) activateHome() (tea.Model, tea.Cmd) {
	switch m.homeIndex {
	case 0:
		m.phase = phaseBrowsing
		m.status = ""
	case 1:
		return m.enterUninstall()
	default:
		m.homeIndex = 0
	}
	return m, nil
}

func (m model) onAskKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "y":
		m.askTold = true
	case "n", "esc", "q":
		m.askName = ""
		m.askTold = false
	}
	return m, nil
}

func (m model) browseKey(k tea.KeyMsg, allowInstall bool) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(k, m.keys.Down):
		if m.cursor < len(m.flat)-1 {
			m.cursor++
		}
	case key.Matches(k, m.keys.Toggle):
		id := m.flat[m.cursor]
		if m.installed[id] && !m.selected[id] && !m.force {
			m.askName = m.itemByID(id).Name
			m.askTold = false
			return m, nil
		}
		m.selected[id] = !m.selected[id]
	case key.Matches(k, m.keys.Install):
		if !allowInstall {
			return m, nil
		}
		if m.countSelected() == 0 {
			m.status = "尚未勾選任何項目"
			return m, nil
		}
		return m.enterConfirm(false)
	case key.Matches(k, m.keys.CatAll):
		m.toggleIDs(m.idsInCategory(m.categoryOf(m.flat[m.cursor])))
	case key.Matches(k, m.keys.All):
		m.toggleIDs(m.flat)
	case key.Matches(k, m.keys.Dry):
		if !allowInstall {
			return m, nil
		}
		if m.countSelected() == 0 {
			m.status = "尚未勾選任何項目"
			return m, nil
		}
		return m.enterConfirm(true)
	case key.Matches(k, m.keys.Top):
		m.cursor = 0
	case key.Matches(k, m.keys.Bottom):
		if len(m.flat) > 0 {
			m.cursor = len(m.flat) - 1
		}
	}
	return m, nil
}

func (m model) idsInCategory(catID string) []catalog.ID {
	var ids []catalog.ID
	for _, c := range m.cats {
		if c.ID != catID {
			continue
		}
		for _, it := range c.Items {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

func (m *model) toggleIDs(ids []catalog.ID) {
	allOn := true
	for _, id := range ids {
		if m.installed[id] {
			continue
		}
		if !m.selected[id] {
			allOn = false
			break
		}
	}
	for _, id := range ids {
		if m.installed[id] {
			continue
		}
		m.selected[id] = !allOn
	}
}

func (m model) enterConfirm(dry bool) (tea.Model, tea.Cmd) {
	m.dryRun = dry
	m.mode = modeInstall
	plans, err := install.ResolveAll(m.selectedIDs(), m.info.Manager)
	if err != nil {
		m.status = err.Error()
		m.dryRun = false
		return m, nil
	}
	steps, err := install.BuildSteps(plans, os.Geteuid(), m.home())
	if err != nil {
		m.status = err.Error()
		m.dryRun = false
		return m, nil
	}
	if len(steps) == 0 {
		m.status = "尚未勾選任何項目"
		m.dryRun = false
		return m, nil
	}
	m.steps = steps
	m.phase = phaseConfirm
	m.status = ""
	return m, nil
}

func (m model) startInstall() (tea.Model, tea.Cmd) {
	if m.dryRun {
		m.results = nil
		for _, st := range m.steps {
			if st.ItemID == "" || st.Kind == install.KindUpdate || st.Kind == install.KindConfig || st.Kind == install.KindLink {
				continue
			}
			m.results = append(m.results, install.ItemResult{
				ID: st.ItemID, Method: st.Method, Status: "skipped",
			})
		}
		m.phase = phaseReport
		m.exitCode = 0
		return m, nil
	}
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
	r := m.deps.Runner
	r.Force = m.force
	r.Exec = teaExec{ch: ch}
	r.EUID = os.Geteuid()
	r.Manager = m.info.Manager
	if r.Home == "" {
		r.Home = m.home()
	}
	if m.deps.Log != nil {
		mgr := m.info.Manager
		logFn := m.deps.Log
		r.Log = func(results []install.ItemResult) error {
			return logFn(mgr, results)
		}
	}
	r.Progress = func(title, status string) {
		ch <- bridgeMsg{kind: kindProgress, title: title, status: status}
	}
	steps := append([]install.Step(nil), m.steps...)
	go func() {
		results, code := r.RunSteps(ctx, steps)
		ch <- bridgeMsg{kind: kindDone, results: results, code: code}
	}()
	return m, listen(ch, runID)
}

func (m model) cancelInstall() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	return m, nil
}
