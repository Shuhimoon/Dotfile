package pm_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/pm"
)

func TestBuildRemove(t *testing.T) {
	cases := []struct {
		mgr  string
		pkgs []string
		euid int
		want string
	}{
		{"apt", []string{"fd-find"}, 1000, "sudo apt-get remove -y fd-find"},
		{"dnf", []string{"python3"}, 1000, "sudo dnf remove -y python3"},
		{"dnf", []string{"gcc", "gcc-c++"}, 0, "dnf remove -y gcc gcc-c++"},
		{"yum", []string{"jq"}, 1000, "sudo yum remove -y jq"},
		{"pacman", []string{"zsh"}, 1000, "sudo pacman -R --noconfirm zsh"},
		{"zypper", []string{"bat"}, 1000, "sudo zypper --non-interactive remove bat"},
		{"apk", []string{"fzf"}, 1000, "sudo apk del fzf"},
		{"xbps", []string{"neovim"}, 1000, "sudo xbps-remove -y neovim"},
		{"brew", []string{"node"}, 501, "brew uninstall node"},
	}
	for _, c := range cases {
		argv, err := pm.BuildRemove(c.mgr, c.pkgs)
		if err != nil {
			t.Fatalf("%s: %v", c.mgr, err)
		}
		argv = pm.WithPrivilege(argv, pm.NeedsPrivilege(c.mgr), c.euid)
		if err := pm.ValidateArgv(argv); err != nil {
			t.Fatalf("%s: %v", c.mgr, err)
		}
		if got := strings.Join(argv, " "); got != c.want {
			t.Errorf("%s: %q want %q", c.mgr, got, c.want)
		}
	}
	bad := []struct {
		mgr  string
		pkgs []string
	}{
		{"dnf", nil},
		{"dnf", []string{"a;rm"}},
		{"dnf", []string{"bash"}},
		{"nix", []string{"jq"}},
	}
	for _, b := range bad {
		if _, err := pm.BuildRemove(b.mgr, b.pkgs); err == nil {
			t.Errorf("%s %v accepted", b.mgr, b.pkgs)
		}
	}
}
