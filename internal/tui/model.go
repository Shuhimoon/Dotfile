package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

type phase int

const (
	phaseDetecting phase = iota
	phaseBrowsing
	// phaseInstalling is the in-TUI progress page while a run is going.
	phaseInstalling
	// phaseReport is the same page once the run ended; any key goes home.
	phaseReport
	phaseFailed
	phaseHome
	phaseUninstall
	// phaseSudo asks for the sudo password (masked) inside the TUI.
	phaseSudo
)

// runMode tells the shared sudo/progress/report phases which flow they belong to.
type runMode int

const (
	modeInstall runMode = iota
	modeUninstall
)

// Deps wires detection and one Runner. EUID is read at install time; zero is root.
// Log receives the detected manager because Runner.Log does not.
type Deps struct {
	Styled bool
	Force  bool
	Uname  string
	Runner install.Runner
	// Exists probes files when planning a removal; nil uses os.Stat.
	Exists func(string) bool
	Log    func(manager string, results []install.ItemResult) error
	// Sudo validates and refreshes sudo credentials; nil runs the real sudo.
	Sudo sudoer
	// Detect re-probes installed tools after a run; nil uses detectInstalled.
	Detect func(manager string) map[catalog.ID]bool
}

type model struct {
	info      platform.Info
	cats      []catalog.Category
	flat      []catalog.ID
	cursor    int
	selected  map[catalog.ID]bool
	phase     phase
	width     int
	height    int
	scroll    int
	installed map[catalog.ID]bool
	force     bool
	homeIndex int
	homeUser  string
	homeHost  string
	homeIP    string
	homeDNS   string
	steps     []install.Step
	results   []install.ItemResult
	events    chan bridgeMsg
	status    string
	dryRun    bool
	// plan is the d-key plan text, shown in white on the report page.
	plan      []string
	exitCode  int
	runID     int
	cancel    context.CancelFunc
	keys      keyMap
	deps      Deps
	mode      runMode
	ucursor   int
	uselected map[catalog.ID]bool
	usteps    []uninstall.Step

	// progress page
	curTitle  string
	stepState []string
	stepRun   int
	stepNext  int
	output    []string
	outCR     bool
	warnings  []string
	canceled  bool

	// sudo page; pw is the masked input and is zeroed on submit or leave.
	pw       []byte
	sudoNote string
	sudoBusy bool
	sudoGen  int
}

func New(info platform.Info) model {
	m := model{
		info:      info,
		cats:      catalog.All(),
		selected:  map[catalog.ID]bool{},
		uselected: map[catalog.ID]bool{},
		width:     80,
		height:    40,
		phase:     phaseBrowsing,
		keys:      newKeyMap(),
		stepRun:   -1,
	}
	for _, c := range m.cats {
		for _, it := range c.Items {
			m.flat = append(m.flat, it.ID)
			m.selected[it.ID] = it.Default
		}
	}
	if info.OS.GOOS == "" && info.Arch.GOARCH == "" && info.Manager == "" && info.OS.SystemText == "" {
		m.phase = phaseDetecting
	}
	return m
}

func SetDeps(m tea.Model, d Deps) tea.Model {
	if mm, ok := m.(model); ok {
		mm.deps = d
		mm.force = d.Force
		return mm
	}
	return m
}

func ExitCode(m tea.Model) int {
	if mm, ok := m.(model); ok {
		return mm.exitCode
	}
	return 1
}

func (m model) Init() tea.Cmd {
	load := func() tea.Msg { return readIdentity() }
	if m.phase != phaseDetecting {
		return load
	}
	uname := m.deps.Uname
	return tea.Batch(load, func() tea.Msg { return liveDetect(uname) })
}

type detectDoneMsg struct {
	Info      platform.Info
	Err       error
	Installed map[catalog.ID]bool
}

type installedMsg struct {
	runID     int
	installed map[catalog.ID]bool
}

func liveDetect(uname string) tea.Msg {
	goos := runtime.GOOS
	if uname == "" {
		uname = platform.UnameM()
	}
	osr, oerr := platform.LoadOSRelease()
	wsl := platform.DetectWSL(os.Getenv("WSL_DISTRO_NAME"), platform.ReadProcVersion())
	info := platform.Info{
		OS:   platform.ClassifyOS(goos, osr, wsl),
		Arch: platform.ClassifyArch(runtime.GOARCH, uname),
	}
	if oerr != nil {
		return detectDoneMsg{Info: info, Err: oerr}
	}
	mgr, err := pm.DetectManager(goos, osr, func(bin string) bool {
		_, e := exec.LookPath(bin)
		return e == nil
	})
	if err != nil {
		return detectDoneMsg{Info: info, Err: err}
	}
	info.Manager = mgr
	return detectDoneMsg{Info: info, Installed: detectInstalled(mgr)}
}

func detectInstalled(manager string) map[catalog.ID]bool {
	plans, err := install.ResolveAll(catalog.IDs(), manager)
	if err != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	return marksFromPlans(plans, home, statExists, xcodeSelectOK(plans))
}

// marksFromPlans marks an item installed when its package probes pass, or when
// a non-package install left its files behind (see localProbes). An empty home
// skips the probes under the home directory.
func marksFromPlans(plans []install.Plan, home string, exists func(string) bool, xcodeOK bool) map[catalog.ID]bool {
	out := map[catalog.ID]bool{}
	for _, p := range plans {
		if install.AlreadySatisfied(p.Method, p.ProbeFiles, p.ProbeGroups, exists, xcodeOK) {
			out[p.ID] = true
			continue
		}
		for _, group := range localProbes(p, home) {
			if allExist(group, exists) {
				out[p.ID] = true
				break
			}
		}
	}
	return out
}

// localProbes lists the files that the non-package install methods leave:
// a GitHub release in ~/.local/bin, the Go tarball in /usr/local/go, and
// rustup in ~/.cargo/bin. Each inner slice must exist in full.
func localProbes(p install.Plan, home string) [][]string {
	methods := []pm.Method{p.Method}
	repo := p.ReleaseRepo
	if p.Fallback != nil {
		methods = append(methods, p.Fallback.Method)
		if repo == "" {
			repo = p.Fallback.ReleaseRepo
		}
	}
	var groups [][]string
	for _, m := range methods {
		switch m {
		case pm.MethodRelease:
			spec, ok := release.SpecFor(repo)
			if ok && home != "" {
				groups = append(groups, []string{filepath.Join(home, ".local", "bin", spec.BinaryName)})
			}
		case pm.MethodGoTarball:
			groups = append(groups, []string{"/usr/local/go/bin/go"})
		case pm.MethodRustup:
			if home != "" {
				bin := filepath.Join(home, ".cargo", "bin")
				groups = append(groups, []string{filepath.Join(bin, "rustc"), filepath.Join(bin, "cargo")})
			}
		}
	}
	return groups
}

func allExist(files []string, exists func(string) bool) bool {
	if exists == nil || len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !exists(f) {
			return false
		}
	}
	return true
}

func statExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func xcodeSelectOK(plans []install.Plan) bool {
	for _, p := range plans {
		if p.Method == pm.MethodXcodeCLT {
			return probeXcodeSelect()
		}
	}
	return false
}

func probeXcodeSelect() bool {
	out, err := exec.Command("xcode-select", "-p").CombinedOutput()
	cls := install.ClassExec("", string(out), 0, err)
	return install.XcodeSelectPresent(cls)
}

// applyInstalled records a finished probe. A nil map is a failed probe: leave defaults selected.
func (m *model) applyInstalled(installed map[catalog.ID]bool) {
	m.installed = installed
	if installed == nil {
		return
	}
	for id, on := range installed {
		if on {
			m.selected[id] = false
		}
	}
}

func refreshInstalled(manager string, runID int) tea.Cmd {
	return func() tea.Msg {
		return installedMsg{runID: runID, installed: detectInstalled(manager)}
	}
}

func (m model) countSelected() int {
	n := 0
	for _, id := range m.flat {
		if m.selected[id] {
			n++
		}
	}
	return n
}

func (m model) selectedIDs() []catalog.ID {
	var ids []catalog.ID
	for _, id := range m.flat {
		if m.selected[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m model) categoryOf(id catalog.ID) string {
	for _, c := range m.cats {
		for _, it := range c.Items {
			if it.ID == id {
				return c.ID
			}
		}
	}
	return ""
}

func (m model) itemByID(id catalog.ID) catalog.Item {
	it, _ := catalog.ItemByID(id)
	return it
}

func (m model) home() string {
	if m.deps.Runner.Home != "" {
		return m.deps.Runner.Home
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}
