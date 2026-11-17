package tui

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
)

func windowsModel(t *testing.T) model {
	t.Helper()
	info := platform.Info{
		OS:      platform.ClassifyOS("windows", platform.OSRelease{PrettyName: "Windows"}, false),
		Arch:    platform.ArchInfo{Family: "x86_64", GOARCH: "amd64"},
		Manager: "scoop",
	}
	home := t.TempDir()
	m := SetDeps(New(info), Deps{GOOS: "windows", Runner: install.Runner{Home: home}}).(model)
	m.width, m.height = 120, 40
	return m
}

func TestWindowsUsesScoopCatalog(t *testing.T) {
	m := windowsModel(t)
	if !reflect.DeepEqual(m.flat, []catalog.ID{"wezterm", "python", "git"}) {
		t.Fatalf("flat %v", m.flat)
	}
	for _, id := range m.flat {
		if !m.selected[id] {
			t.Fatalf("%s not selected by default", id)
		}
	}
	m.phase = phaseBrowsing
	if v := render(m, false); !strings.Contains(v, "Windows（Scoop）") || strings.Contains(v, "ripgrep") {
		t.Fatalf("windows list\n%s", v)
	}
	lin := SetDeps(New(fedoraInfo()), Deps{GOOS: "linux"}).(model)
	if lin.cats[0].ID != "basics" || len(lin.flat) == 3 {
		t.Fatal("linux catalog changed")
	}
}

func TestWindowsInstallPlan(t *testing.T) {
	m := windowsModel(t)
	next, _ := m.startInstall(true)
	m = next.(model)
	if installNeedsSudo(m.steps) {
		t.Fatal("sudo on windows")
	}
	var argv []string
	for _, st := range m.steps {
		argv = append(argv, strings.Join(st.Argv, " "))
		switch st.Kind {
		case install.KindConfig, install.KindLink, install.KindUpdate, install.KindService:
			t.Fatalf("linux-only step on windows: %s", st.Kind)
		}
	}
	want := []string{"scoop bucket add extras", "scoop install extras/wezterm", "scoop install python", "scoop install git"}
	if !reflect.DeepEqual(argv[1:], want) || !strings.HasPrefix(argv[0], "powershell ") {
		t.Fatalf("argv %v", argv)
	}
	plan := strings.Join(m.plan, "\n")
	if !strings.Contains(plan, "沒有 Scoop 時執行：powershell") || !strings.Contains(plan, "scoop install extras/wezterm") {
		t.Fatalf("plan\n%s", plan)
	}
}

func TestWindowsUninstallPlan(t *testing.T) {
	m := windowsModel(t)
	m.applyInstalled(map[catalog.ID]bool{"python": true, "git": true})
	next, _ := m.enterUninstall()
	m = next.(model)
	if got := m.removableIDs(); !reflect.DeepEqual(got, []catalog.ID{"python", "git"}) {
		t.Fatalf("removable %v", got)
	}
	m.uselected = map[catalog.ID]bool{"python": true, "git": true}
	next, _ = m.startUninstall(true)
	m = next.(model)
	if len(m.warnings) != 0 {
		t.Fatalf("warnings on windows: %v", m.warnings)
	}
	plan := strings.Join(m.plan, "\n")
	for _, s := range []string{"scoop uninstall python", "scoop uninstall git", "ssh-config git"} {
		if !strings.Contains(plan, s) {
			t.Fatalf("plan lacks %q\n%s", s, plan)
		}
	}
	if uninstallNeedsSudo(m.usteps) {
		t.Fatal("sudo on windows")
	}
}

func TestWindowsGitKeyAfterInstall(t *testing.T) {
	m := windowsModel(t)
	m.deps.Detect = func(string) map[catalog.ID]bool { return nil }
	m.mode = modeInstall
	m.finishRun([]install.ItemResult{{ID: "scoop", Status: "ok"}, {ID: "git", Status: "ok"}}, 0)
	if !m.gitPending {
		t.Fatal("key setup not offered after scoop install git")
	}
	if got := m.sshLayout().IdentRel; got != "~/.ssh/" {
		t.Fatalf("identity dir %q", got)
	}
}

func TestScoopArgvUsesShimWhenNotOnPath(t *testing.T) {
	t.Setenv("SCOOP", filepath.Join("X", "scoop"))
	shim := filepath.Join("X", "scoop", "shims", "scoop.cmd")
	miss := func(string) (string, error) { return "", errors.New("no") }
	hit := func(string) (string, error) { return "scoop", nil }
	got := scoopArgv([]string{"scoop", "install", "git"}, miss, func(p string) bool { return p == shim })
	if !reflect.DeepEqual(got, []string{shim, "install", "git"}) {
		t.Fatalf("%v", got)
	}
	if got := scoopArgv([]string{"scoop", "install", "git"}, hit, func(string) bool { return true }); got[0] != "scoop" {
		t.Fatalf("%v", got)
	}
	if got := scoopArgv([]string{"dnf", "install", "git"}, miss, func(string) bool { return true }); got[0] != "dnf" {
		t.Fatalf("%v", got)
	}
}
