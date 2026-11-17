package pm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/platform"
	"github.com/shuhi/dotfiles/internal/pm"
)

func has(bins ...string) func(string) bool {
	set := map[string]bool{}
	for _, b := range bins {
		set[b] = true
	}
	return func(bin string) bool { return set[bin] }
}

func TestDetectManager(t *testing.T) {
	dnf, err := pm.DetectManager("linux", platform.OSRelease{ID: "fedora"}, has("dnf", "yum"))
	if err != nil || dnf != "dnf" {
		t.Fatalf("fedora = %s, %v", dnf, err)
	}
	_, err = pm.DetectManager("linux", platform.OSRelease{ID: "fedora"}, has("yum"))
	if err == nil || strings.Contains(err.Error(), "yum install") {
		t.Fatalf("fedora without dnf should fail, got %v", err)
	}
	if err == nil {
		t.Fatal("expected error")
	}

	got, err := pm.DetectManager("linux", platform.OSRelease{ID: "rhel"}, has("dnf", "yum"))
	if err != nil || got != "dnf" {
		t.Fatalf("rhel dnf = %s %v", got, err)
	}
	got, err = pm.DetectManager("linux", platform.OSRelease{ID: "centos"}, has("yum"))
	if err != nil || got != "yum" {
		t.Fatalf("rhel yum = %s %v", got, err)
	}

	got, err = pm.DetectManager("linux", platform.OSRelease{ID: "debian"}, has("apt-get"))
	if err != nil || got != "apt" {
		t.Fatalf("debian = %s %v", got, err)
	}
	got, err = pm.DetectManager("linux", platform.OSRelease{ID: "ubuntu", IDLike: "debian"}, has("apt-get"))
	if err != nil || got != "apt" {
		t.Fatalf("ubuntu = %s %v", got, err)
	}

	got, err = pm.DetectManager("darwin", platform.OSRelease{}, has("brew"))
	if err != nil || got != "brew" {
		t.Fatalf("darwin = %s %v", got, err)
	}
	_, err = pm.DetectManager("darwin", platform.OSRelease{}, has())
	if !errors.Is(err, pm.ErrBrewMissing) {
		t.Fatalf("darwin missing brew: %v", err)
	}

	_, err = pm.DetectManager("linux", platform.OSRelease{ID: "custom"}, has("apt-get", "dnf"))
	if err == nil || !strings.Contains(err.Error(), "apt-get") || !strings.Contains(err.Error(), "dnf") {
		t.Fatalf("ambiguous: %v", err)
	}
	if errors.Is(err, pm.ErrManagerMissing) {
		t.Fatal("ambiguous reported as missing")
	}

	if mgr, err := pm.DetectManager("windows", platform.OSRelease{}, has()); err != nil || mgr != pm.Scoop {
		t.Fatalf("windows: %q %v", mgr, err)
	}
	_, err = pm.DetectManager("freebsd", platform.OSRelease{}, has("dnf"))
	if err == nil || !strings.Contains(err.Error(), "linux") || !strings.Contains(err.Error(), "darwin") {
		t.Fatalf("freebsd: %v", err)
	}

	_, err = pm.DetectManager("linux", platform.OSRelease{ID: "nixos"}, has("nix"))
	if !errors.Is(err, pm.ErrManagerMissing) || !strings.Contains(err.Error(), "不支援的套件管理程式，本次不會安裝任何套件") {
		t.Fatalf("nixos: %v", err)
	}

	got, err = pm.DetectManager("linux", platform.OSRelease{ID: "arch"}, has("pacman"))
	if err != nil || got != "pacman" {
		t.Fatalf("arch = %s %v", got, err)
	}
	got, err = pm.DetectManager("linux", platform.OSRelease{ID: "alpine"}, has("apk"))
	if err != nil || got != "apk" {
		t.Fatalf("alpine = %s %v", got, err)
	}
	_, err = pm.DetectManager("linux", platform.OSRelease{ID: "debian"}, has())
	if err == nil || !strings.Contains(err.Error(), "debian") || !strings.Contains(err.Error(), "apt-get") {
		t.Fatalf("missing apt-get: %v", err)
	}
}
