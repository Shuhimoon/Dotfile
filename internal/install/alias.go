package install

import (
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/safepath"
)

type Symlink struct {
	Link   string
	Target string
}

// AliasSymlinks reports Debian command-name links. stat only checks existence.
func AliasSymlinks(home string, stat func(path string) bool) []Symlink {
	if stat == nil {
		return nil
	}
	var out []Symlink
	if !stat("/usr/bin/fd") && stat("/usr/bin/fdfind") {
		out = append(out, Symlink{
			Link:   filepath.Join(home, ".local", "bin", "fd"),
			Target: "/usr/bin/fdfind",
		})
	}
	if !stat("/usr/bin/bat") && stat("/usr/bin/batcat") {
		out = append(out, Symlink{
			Link:   filepath.Join(home, ".local", "bin", "bat"),
			Target: "/usr/bin/batcat",
		})
	}
	return out
}

// ApplyAlias creates a Debian fd/bat symlink after a package install.
// An already-correct symlink is a no-op and does not set NeedsLocalBin.
func ApplyAlias(home string, res *ItemResult, exists func(string) bool) {
	if res == nil || home == "" || !successStatus(res.Status) {
		return
	}
	if res.Method != "package" || (res.ID != "fd" && res.ID != "bat") {
		return
	}
	for _, sl := range AliasSymlinks(home, exists) {
		base := filepath.Base(sl.Link)
		if res.ID == "fd" && base != "fd" {
			continue
		}
		if res.ID == "bat" && base != "bat" {
			continue
		}
		if err := safepath.RejectEscapingSymlink(sl.Link, home); err != nil {
			res.Status = "failed"
			res.ErrText = err.Error()
			return
		}
		created, err := release.PlaceSymlink(sl.Link, sl.Target, false)
		if err != nil {
			res.Status = "failed"
			res.ErrText = err.Error()
			return
		}
		if created {
			res.NeedsLocalBin = true
		}
	}
}

// FormatResult is the one-line report used by the TUI and the CLI.
func FormatResult(res ItemResult) string {
	s := string(res.ID) + "  " + res.Status
	if res.ErrText != "" {
		s += "  " + res.ErrText
	}
	if res.Note != "" {
		s += "  " + res.Note
	}
	return strings.TrimRight(s, " ")
}
