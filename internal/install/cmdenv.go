package install

import (
	"os"
	"path/filepath"
	"strings"
)

// rootHomeEnv mirrors sudouser.RootHomeEnv (no import, to keep install free
// of the sudo package).
const rootHomeEnv = "DOTFILES_SUDO_ROOT_HOME"

// systemCommands run as root on the system; under `sudo ./tools` they keep
// root's HOME so package scriptlets do not write into the user's home.
var systemCommands = map[string]bool{
	"dnf": true, "dnf5": true, "yum": true, "apt-get": true, "apt": true, "dpkg": true,
	"pacman": true, "zypper": true, "apk": true, "xbps-install": true, "xbps-remove": true,
	"rpm": true, "systemctl": true, "groupadd": true, "usermod": true,
}

// CommandEnv is the environment for argv: nil (inherit) normally; with HOME
// set back to root's home for system commands during a `sudo ./tools` run.
func CommandEnv(argv []string) []string {
	rootHome := os.Getenv(rootHomeEnv)
	if rootHome == "" || len(argv) == 0 || !systemCommands[filepath.Base(argv[0])] {
		return nil
	}
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "HOME=") {
			out = append(out, kv)
		}
	}
	return append(out, "HOME="+rootHome)
}
