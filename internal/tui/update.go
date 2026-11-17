package tui

import (
	"os"

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
	case sudoCheckMsg:
		return m.onSudoCheck(msg)
	case sudoValidMsg:
		return m.onSudoValid(msg)
	}
	return m, nil
}

func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseDetecting:
		return m.homeKey(k, false)
	case phaseHome:
		return m.homeKey(k, true)
	case phaseUninstall:
		return m.uninstallKey(k)
	case phaseSudo:
		return m.sudoKey(k)
	case phaseGitKey:
		return m.gitKeyKey(k)
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
	case phaseInstalling:
		if k.String() == "ctrl+c" {
			return m.cancelRun()
		}
	case phaseReport:
		// The d-key plan goes back to its list; after a real run any key,
		// q included, returns to the home page.
		if m.dryRun {
			return m.backToList()
		}
		if m.gitPending {
			return m.enterGitKey()
		}
		return m.goHome()
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
		m.mode = modeInstall
		m.status = ""
	case 1:
		return m.enterUninstall()
	default:
		m.homeIndex = 0
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
		// An installed tool can be picked too: it is downloaded again.
		id := m.flat[m.cursor]
		m.selected[id] = !m.selected[id]
	case key.Matches(k, m.keys.Install):
		if !allowInstall {
			return m, nil
		}
		if m.countSelected() == 0 {
			m.status = "尚未勾選任何項目"
			return m, nil
		}
		return m.startInstall(false)
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
		return m.startInstall(true)
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

// toggleIDs is a and A: tick every target unless all are ticked, then clear
// them. Installed tools are left alone so a re-download is never picked by
// accident, except when every target is installed: then a/A toggles those
// (they are downloaded again) and the status line says so, instead of the
// key silently doing nothing.
func (m *model) toggleIDs(ids []catalog.ID) {
	var targets []catalog.ID
	for _, id := range ids {
		if !m.installed[id] {
			targets = append(targets, id)
		}
	}
	m.status = ""
	if len(targets) == 0 {
		targets = ids
		if len(ids) > 0 {
			m.status = "這些都已安裝：勾選的會重新下載"
		}
	}
	allOn := len(targets) > 0
	for _, id := range targets {
		if !m.selected[id] {
			allOn = false
			break
		}
	}
	for _, id := range targets {
		m.selected[id] = !allOn
	}
}

// startInstall goes straight from the list to the work: no y/n step.
// dry shows the plan on the report page without running anything.
func (m model) startInstall(dry bool) (tea.Model, tea.Cmd) {
	m.mode = modeInstall
	m.warnings = nil
	plans, err := install.ResolveAll(m.selectedIDs(), m.info.Manager)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	steps, err := install.BuildSteps(plans, os.Geteuid(), m.home())
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	if len(steps) == 0 {
		m.status = "尚未勾選任何項目"
		return m, nil
	}
	m.steps = steps
	m.status = ""
	m.dryRun = dry
	if dry {
		return m.dryReport(), nil
	}
	return m.needSudo(installNeedsSudo(steps))
}

func (m model) dryReport() model {
	m.plan = installPlan(m.steps, m.home())
	m.results = nil
	m.output = nil
	m.canceled = false
	m.stepRun = -1
	m.stepState = make([]string, len(m.steps))
	for i, st := range m.steps {
		m.stepState[i] = "skipped"
		if st.ItemID == "" || st.Kind == install.KindUpdate || st.Kind == install.KindConfig || st.Kind == install.KindLink || st.Kind == install.KindService {
			continue
		}
		m.results = append(m.results, install.ItemResult{
			ID: st.ItemID, Method: st.Method, Status: "skipped",
		})
	}
	m.phase = phaseReport
	m.exitCode = 0
	return m
}
