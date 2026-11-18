//go:build windows

package sudouser

import "io/fs"

// Windows has no sudo/uid: nothing to give back.
func fileOwner(fs.FileInfo) (int, bool) { return 0, false }
