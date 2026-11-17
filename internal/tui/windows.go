package tui

import (
	"os"
	"os/exec"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
	"github.com/shuhi/dotfiles/internal/pm"
)

// windows is true when the TUI runs the Scoop catalog (deps.GOOS, else runtime.GOOS).
func (m model) windows() bool { return m.goos() == "windows" }

// useCatalog loads the catalog for goos and its default selection. It is a
// no-op when that catalog is already loaded.
func (m *model) useCatalog(goos string) {
	cats := catalog.ForOS(goos)
	if len(m.cats) > 0 && len(cats) > 0 && m.cats[0].ID == cats[0].ID {
		return
	}
	m.cats = cats
	m.flat = nil
	m.selected = map[catalog.ID]bool{}
	m.cursor = 0
	for _, c := range m.cats {
		for _, it := range c.Items {
			m.flat = append(m.flat, it.ID)
			m.selected[it.ID] = it.Default
		}
	}
}

// winEnv points the Windows step builder at %SCOOP% (or ~\scoop) and PATH.
func winEnv(home string) install.WinEnv {
	return install.WinEnv{
		ScoopRoot: pm.ScoopRoot(home, os.Getenv("SCOOP")),
		Look: func(bin string) (string, bool) {
			p, err := exec.LookPath(bin)
			return p, err == nil
		},
	}
}

func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// windowsDetect is liveDetect on Windows: there is no /etc/os-release, and
// Scoop is the manager even before it is installed (the run bootstraps it).
func windowsDetect(goarch string) detectDoneMsg {
	osInfo := platform.ClassifyOS("windows", platform.OSRelease{PrettyName: "Windows"}, false)
	info := platform.Info{
		OS:      osInfo,
		Arch:    platform.ClassifyArch(goarch, ""),
		Manager: pm.Scoop,
	}
	return detectDoneMsg{Info: info, Installed: install.WindowsInstalled(winEnv(userHome()), statExists)}
}

// scoopArgv runs "scoop ..." through the shim when scoop is not on this
// process's PATH yet (right after the bootstrap step installed it).
func scoopArgv(argv []string, look func(string) (string, error), exists func(string) bool) []string {
	if len(argv) == 0 || argv[0] != "scoop" {
		return argv
	}
	if _, err := look("scoop"); err == nil {
		return argv
	}
	for _, p := range pm.ScoopCommands(pm.ScoopRoot(userHome(), os.Getenv("SCOOP")))[:1] {
		if exists(p) {
			return append([]string{p}, argv[1:]...)
		}
	}
	return argv
}

// installSteps is the install plan for the selected items on this platform.
func (m model) installSteps() ([]install.Step, error) {
	if m.windows() {
		return install.BuildWindowsSteps(m.selectedIDs(), winEnv(m.home()))
	}
	plans, err := install.ResolveAll(m.selectedIDs(), m.info.Manager)
	if err != nil {
		return nil, err
	}
	return install.BuildSteps(plans, os.Geteuid(), m.home())
}
