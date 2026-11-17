package pm

import (
	"fmt"
	"regexp"
)

var userNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}\$?$`)

// DockerSetup is what runs after Docker is installed on Linux, in order:
// make sure the docker group exists, add user to it, and (only when systemd
// runs) enable and start the service. user "" skips the group membership
// (root needs none). Every command is fixed; only user varies and is checked.
func DockerSetup(user string, euid int, systemd bool) ([][]string, error) {
	if user != "" && !userNameRe.MatchString(user) {
		return nil, fmt.Errorf("不合法的使用者名稱 %q", user)
	}
	cmds := [][]string{{"groupadd", "-f", "docker"}}
	if user != "" {
		cmds = append(cmds, []string{"usermod", "-aG", "docker", user})
	}
	if systemd {
		cmds = append(cmds, []string{"systemctl", "enable", "--now", "docker.service"})
	}
	for i, c := range cmds {
		cmds[i] = WithPrivilege(c, true, euid)
	}
	return cmds, nil
}
