package pm_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

func TestDockerSetup(t *testing.T) {
	join := func(c [][]string) string {
		var s []string
		for _, a := range c {
			s = append(s, strings.Join(a, " "))
		}
		return strings.Join(s, " | ")
	}
	cmds, err := pm.DockerSetup("shuhi", 1000, true)
	if err != nil || join(cmds) != "sudo groupadd -f docker | sudo usermod -aG docker shuhi | sudo systemctl enable --now docker.service" {
		t.Fatalf("%q %v", join(cmds), err)
	}
	cmds, _ = pm.DockerSetup("shuhi", 1000, false)
	if join(cmds) != "sudo groupadd -f docker | sudo usermod -aG docker shuhi" {
		t.Fatalf("no systemd %q", join(cmds))
	}
	cmds, _ = pm.DockerSetup("", 0, true)
	if join(cmds) != "groupadd -f docker | systemctl enable --now docker.service" {
		t.Fatalf("root %q", join(cmds))
	}
	for _, bad := range []string{"a b", "x;rm", "-o", "$(id)", "Root", strings.Repeat("a", 40)} {
		if _, err := pm.DockerSetup(bad, 1000, true); err == nil {
			t.Errorf("accepted user %q", bad)
		}
	}
}

func TestContainerPackages(t *testing.T) {
	want := map[string][2]string{
		"apt":    {"docker.io", "podman"},
		"dnf":    {"moby-engine docker-compose", "podman"},
		"yum":    {"docker", "podman"},
		"pacman": {"docker docker-compose", "podman"},
		"zypper": {"docker docker-compose", "podman"},
		"apk":    {"docker docker-cli-compose", "podman"},
		"xbps":   {"docker docker-compose", "podman"},
		"brew":   {"docker docker-compose", "podman"},
	}
	for mgr, w := range want {
		for i, id := range []string{"docker", "podman"} {
			p, err := pm.Resolve(catalog.ID(id), mgr)
			if err != nil || p.Method != pm.MethodPackage || strings.Join(p.Packages, " ") != w[i] {
				t.Fatalf("%s %s: %+v %v", mgr, id, p, err)
			}
		}
	}
}
