package install

import (
	"context"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/shuhi/dotfiles/internal/pm"
)

// systemdDir exists only while systemd is PID 1 (sd_booted). WSL without
// systemd=true in /etc/wsl.conf does not have it.
const systemdDir = "/run/systemd/system"

const (
	noteDockerGroup   = "已加入 docker 群組（重新登入或執行 newgrp docker 後生效）"
	noteDockerService = "docker 服務已啟用並啟動"
	noteNoSystemd     = "沒有 systemd（例如沒開 systemd 的 WSL），沒有啟動 docker 服務；可在 /etc/wsl.conf 加 [boot] systemd=true，或手動執行 sudo dockerd"
)

// doService runs pm.DockerSetup after a successful Docker install. A failed
// command marks the step failed and is noted on the docker result; the
// package itself stays installed. Returns the step status.
func (r *Runner) doService(ctx context.Context, st Step, results []ItemResult) (ItemResult, string, bool) {
	cur, ok := findResult(results, st.ItemID)
	if !ok || !successStatus(cur.Status) {
		return cur, "skipped", false
	}
	name := r.dockerUser()
	systemd := r.exists(systemdDir)
	cmds, err := pm.DockerSetup(name, r.EUID, systemd)
	if err != nil {
		cur.Note = joinNote(cur.Note, err.Error())
		return cur, "failed", false
	}
	for _, argv := range cmds {
		cls := r.execClass(ctx, argv, "")
		if cls.Canceled {
			return cur, "", true
		}
		if cls.ExecErr != nil || cls.ErrText != "" || cls.ExitCode != 0 {
			msg := cls.ErrText
			if msg == "" {
				msg = "exit " + strconv.Itoa(cls.ExitCode)
			}
			cur.Note = joinNote(cur.Note, "docker 設定失敗（"+strings.Join(argv, " ")+"）："+msg)
			return cur, "failed", false
		}
	}
	if name != "" {
		cur.Note = joinNote(cur.Note, noteDockerGroup)
	}
	if systemd {
		cur.Note = joinNote(cur.Note, noteDockerService)
	} else {
		cur.Note = joinNote(cur.Note, noteNoSystemd)
	}
	return cur, "ok", false
}

func (r *Runner) dockerUser() string {
	if r.User != nil {
		return r.User()
	}
	if r.EUID == 0 {
		return os.Getenv("SUDO_USER")
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "；" + b
}
