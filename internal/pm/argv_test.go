package pm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/pm"
)

func TestBuildArgvNoShell(t *testing.T) {
	for _, m := range pm.Managers() {
		argv, err := pm.BuildInstall(m, []string{"git"})
		if err != nil {
			t.Fatal(m, err)
		}
		assertNoShell(t, argv)
		upd, err := pm.BuildUpdate(m)
		if err != nil {
			t.Fatal(m, err)
		}
		assertNoShell(t, upd)
	}
}

func assertNoShell(t *testing.T, argv []string) {
	t.Helper()
	for _, a := range argv {
		if a == "sh" || a == "bash" || a == "-c" {
			t.Fatalf("shell token in %v", argv)
		}
	}
}

func TestAptArgv(t *testing.T) {
	got, err := pm.BuildInstall("apt", []string{"git", "curl"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apt-get", "install", "-y", "--no-install-recommends", "git", "curl"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("apt")
	if err != nil || !slices.Equal(upd, []string{"apt-get", "update"}) {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestDnfArgv(t *testing.T) {
	got, err := pm.BuildInstall("dnf", []string{"gcc", "gcc-c++"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dnf", "install", "-y", "gcc", "gcc-c++"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("dnf")
	if err != nil || upd != nil {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestYumArgv(t *testing.T) {
	got, err := pm.BuildInstall("yum", []string{"vim-enhanced"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"yum", "install", "-y", "vim-enhanced"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("yum")
	if err != nil || upd != nil {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestPacmanArgv(t *testing.T) {
	got, err := pm.BuildInstall("pacman", []string{"base-devel"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pacman", "-S", "--needed", "--noconfirm", "base-devel"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("pacman")
	if err != nil || !slices.Equal(upd, []string{"pacman", "-Sy", "--noconfirm"}) {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestZypperArgv(t *testing.T) {
	got, err := pm.BuildInstall("zypper", []string{"gcc", "gcc-c++"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zypper", "--non-interactive", "install", "--no-recommends", "gcc", "gcc-c++"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("zypper")
	if err != nil || !slices.Equal(upd, []string{"zypper", "--non-interactive", "refresh"}) {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestApkArgv(t *testing.T) {
	got, err := pm.BuildInstall("apk", []string{"build-base"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apk", "add", "build-base"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("apk")
	if err != nil || !slices.Equal(upd, []string{"apk", "update"}) {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestXbpsArgv(t *testing.T) {
	got, err := pm.BuildInstall("xbps", []string{"base-devel"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"xbps-install", "-y", "-S", "base-devel"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("xbps")
	if err != nil || upd != nil {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestBrewArgv(t *testing.T) {
	got, err := pm.BuildInstall("brew", []string{"node"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"brew", "install", "node"}
	if !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
	upd, err := pm.BuildUpdate("brew")
	if err != nil || upd != nil {
		t.Fatalf("update %v %v", upd, err)
	}
}

func TestSudoPrefix(t *testing.T) {
	argv, err := pm.BuildInstall("dnf", []string{"git"})
	if err != nil {
		t.Fatal(err)
	}
	got := pm.WithPrivilege(argv, pm.NeedsPrivilege("dnf"), 1000)
	want := []string{"sudo", "dnf", "install", "-y", "git"}
	if !slices.Equal(got, want) {
		t.Fatalf("euid 1000 %v", got)
	}
	got = pm.WithPrivilege(argv, pm.NeedsPrivilege("dnf"), 0)
	if got[0] == "sudo" {
		t.Fatalf("euid 0 added sudo: %v", got)
	}
}

func TestBrewNeverSudo(t *testing.T) {
	argv, err := pm.BuildInstall("brew", []string{"git"})
	if err != nil {
		t.Fatal(err)
	}
	if pm.NeedsPrivilege("brew") {
		t.Fatal("brew should not need privilege")
	}
	got := pm.WithPrivilege(argv, pm.NeedsPrivilege("brew"), 1000)
	if len(got) == 0 || got[0] == "sudo" || strings.Contains(strings.Join(got, " "), "sudo") {
		t.Fatalf("%v", got)
	}
}

func TestArgvRejectsMetacharacters(t *testing.T) {
	for _, bad := range []string{"git;rm", "git rm", "git\nrm", "git|rm", "git&rm", "git$HOME", "git`rm`"} {
		if _, err := pm.BuildInstall("apt", []string{bad}); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
