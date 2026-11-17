//go:build windows

package sshkey

import (
	"os"
	"os/exec"
)

// restrict is best effort on Windows: Go's 0600 only sets the read-only bit,
// so icacls drops inherited ACLs and grants the current user full control.
// OpenSSH for Windows refuses keys readable by other users.
func restrict(path string) {
	user := os.Getenv("USERNAME")
	if user == "" {
		return
	}
	_ = exec.Command("icacls", path, "/inheritance:r", "/grant:r", user+":F").Run()
}
