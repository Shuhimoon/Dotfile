package install_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

func dockerSteps(t *testing.T, mgr string, euid int) []install.Step {
	t.Helper()
	plans, err := install.ResolveAll([]catalog.ID{"podman", "docker"}, mgr)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, euid, "/home/u")
	if err != nil {
		t.Fatal(err)
	}
	return steps
}

func titles(steps []install.Step) string {
	var s []string
	for _, st := range steps {
		s = append(s, st.Title)
	}
	return strings.Join(s, ",")
}

func TestDockerServiceStepOnlyOnLinux(t *testing.T) {
	if got := titles(dockerSteps(t, "dnf", 1000)); got != "install docker,install podman,service docker,shell-rc" {
		t.Fatalf("dnf %q", got)
	}
	if got := titles(dockerSteps(t, "apt", 1000)); got != "update,install docker,install podman,service docker,shell-rc" {
		t.Fatalf("apt %q", got)
	}
	for _, mgr := range []string{"yum", "pacman", "zypper", "apk", "xbps"} {
		if got := titles(dockerSteps(t, mgr, 1000)); !strings.HasSuffix(got, "install docker,install podman,service docker,shell-rc") {
			t.Fatalf("%s %q", mgr, got)
		}
	}
	if got := titles(dockerSteps(t, "brew", 501)); strings.Contains(got, "service") {
		t.Fatalf("brew %q", got)
	}
	steps := dockerSteps(t, "dnf", 1000)
	if strings.Join(steps[0].Argv, " ") != "sudo dnf install -y moby-engine docker-compose" {
		t.Fatalf("argv %q", steps[0].Argv)
	}
}

func runDocker(t *testing.T, systemd bool, fail string, probeHit bool) ([]install.ItemResult, []string, *fakeExec) {
	t.Helper()
	fe := &fakeExec{fn: func(argv []string) (string, int, error) {
		if fail != "" && strings.Contains(strings.Join(argv, " "), fail) {
			return "boom", 1, nil
		}
		return "", 0, nil
	}}
	var notes []string
	r := &install.Runner{
		Exec:    fe,
		EUID:    1000,
		Home:    t.TempDir(),
		Manager: "dnf",
		User:    func() string { return "shuhi" },
		Exists: func(p string) bool {
			if p == "/run/systemd/system" {
				return systemd
			}
			return probeHit && p == "/usr/bin/docker"
		},
		Progress: func(title, status string) {
			if status != "" {
				notes = append(notes, title+"="+status)
			}
		},
	}
	plans, _ := install.ResolveAll([]catalog.ID{"docker"}, "dnf")
	steps, _ := install.BuildSteps(plans, 1000, r.Home)
	results, _ := r.RunSteps(context.Background(), steps)
	return results, notes, fe
}

func TestDockerServiceSetsGroupAndService(t *testing.T) {
	results, notes, fe := runDocker(t, true, "", false)
	var cmds []string
	for _, c := range fe.calls {
		cmds = append(cmds, strings.Join(c, " "))
	}
	want := "sudo dnf install -y moby-engine docker-compose|sudo groupadd -f docker|sudo usermod -aG docker shuhi|sudo systemctl enable --now docker.service"
	if strings.Join(cmds, "|") != want {
		t.Fatalf("calls %q", cmds)
	}
	if strings.Join(notes, ",") != "install docker=ok,service docker=ok,shell-rc=ok" {
		t.Fatalf("notes %v", notes)
	}
	if len(results) != 1 || results[0].Status != "ok" || !strings.Contains(results[0].Note, "docker 群組") || !strings.Contains(results[0].Note, "已啟用並啟動") {
		t.Fatalf("%+v", results)
	}
}

func TestDockerServiceWithoutSystemd(t *testing.T) {
	results, _, fe := runDocker(t, false, "", true) // already installed: satisfied
	for _, c := range fe.calls {
		if strings.Contains(strings.Join(c, " "), "systemctl") {
			t.Fatal("systemctl without systemd")
		}
	}
	if results[0].Status != "satisfied" || !strings.Contains(results[0].Note, "沒有 systemd") {
		t.Fatalf("%+v", results[0])
	}
}

func TestDockerServiceFailureKeepsPackage(t *testing.T) {
	results, notes, _ := runDocker(t, true, "usermod", false)
	if results[0].Status != "ok" || !strings.Contains(results[0].Note, "docker 設定失敗") {
		t.Fatalf("%+v", results[0])
	}
	if notes[len(notes)-2] != "service docker=failed" {
		t.Fatalf("notes %v", notes)
	}
	// A failed package install skips the service step entirely.
	results, notes, fe := runDocker(t, true, "dnf install", false)
	if results[0].Status != "failed" || notes[len(notes)-2] != "service docker=skipped" || len(fe.calls) != 1 {
		t.Fatalf("%+v %v %v", results, notes, fe.calls)
	}
}

// Docker is split per platform: each manager installs its own distro package.
func TestDockerInstallArgvPerPlatform(t *testing.T) {
	want := map[string]string{
		"apt":    "sudo apt-get install -y --no-install-recommends docker.io",                   // Debian / Ubuntu / WSL Ubuntu
		"dnf":    "sudo dnf install -y moby-engine docker-compose",                              // Fedora / WSL Fedora
		"yum":    "sudo yum install -y docker",                                                  // CentOS 7 / Amazon Linux 2
		"pacman": "sudo pacman -S --needed --noconfirm docker docker-compose",                   // Arch
		"zypper": "sudo zypper --non-interactive install --no-recommends docker docker-compose", // openSUSE
		"apk":    "sudo apk add docker docker-cli-compose",                                      // Alpine
		"xbps":   "sudo xbps-install -y -S docker docker-compose",                               // Void
		"brew":   "brew install docker docker-compose",                                          // macOS (CLI only)
	}
	for mgr, w := range want {
		found := false
		for _, s := range dockerSteps(t, mgr, 1000) {
			if s.Title == "install docker" {
				found = true
				if got := strings.Join(s.Argv, " "); got != w {
					t.Errorf("%s: %q, want %q", mgr, got, w)
				}
			}
		}
		if !found {
			t.Errorf("%s: no install docker step", mgr)
		}
	}
}
