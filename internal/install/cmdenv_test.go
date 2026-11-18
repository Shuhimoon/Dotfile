package install_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/install"
)

func TestCommandEnvRootHomeForSystemCommands(t *testing.T) {
	t.Setenv("HOME", "/home/shuhi")
	t.Setenv("DOTFILES_SUDO_ROOT_HOME", "")
	if env := install.CommandEnv([]string{"dnf", "install", "-y", "vim"}); env != nil {
		t.Fatal("normal run inherits the environment")
	}
	t.Setenv("DOTFILES_SUDO_ROOT_HOME", "/root")
	home := func(env []string) string {
		h := ""
		for _, kv := range env {
			if strings.HasPrefix(kv, "HOME=") {
				h = strings.TrimPrefix(kv, "HOME=")
			}
		}
		return h
	}
	for _, argv := range [][]string{{"dnf", "install", "-y", "vim"}, {"/usr/bin/apt-get", "update"}, {"systemctl", "enable", "docker"}} {
		if got := home(install.CommandEnv(argv)); got != "/root" {
			t.Fatalf("%v HOME=%q", argv, got)
		}
	}
	if env := install.CommandEnv([]string{"git", "clone", "x"}); env != nil {
		t.Fatal("user commands keep the user's HOME")
	}
}
