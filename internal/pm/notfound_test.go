package pm_test

import (
	"errors"
	"testing"

	"github.com/shuhi/dotfiles/internal/pm"
)

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		mgr, out string
		code     int
		wantNF   bool
	}{
		{"apt", "E: Unable to locate package starship", 100, true},
		{"apt", "E: Package has no installation candidate", 100, true},
		{"apt", "sudo: a password is required", 100, false},
		{"apt", "dpkg locked", 100, false},
		{"dnf", "No match for argument: starship", 1, true},
		{"dnf", "Unable to find a match: starship", 1, true},
		{"dnf", "Nothing to do.", 1, false},
		{"dnf", "sudo: a password is required\nNo match for argument", 1, false},
		{"yum", "No match for argument: whois", 1, true},
		{"yum", "No package whois available.", 1, true},
		{"yum", "Error: disk failure", 1, false},
		{"pacman", "error: target not found: zellij", 1, true},
		{"pacman", "error: failed to init transaction", 1, false},
		{"zypper", "", 104, true},
		{"zypper", "a terminal is required", 104, false},
		{"zypper", "refresh failed", 1, false},
		{"apk", "ERROR: unable to select packages", 1, true},
		{"apk", "no such package: foo", 2, true},
		{"apk", "ERROR: world update failed", 1, false},
		{"xbps", "pkg not found in repository pool", 2, true},
		{"xbps", "transaction error", 1, false},
		{"brew", "Error: No available formula with the name \"zellij\".", 1, true},
		{"brew", "No formulae found for missing.", 1, true},
		{"brew", "sudo: a password is required", 1, false},
		{"brew", "Error: curl failed", 1, false},
	}
	for _, tc := range cases {
		err := pm.ClassifyFailure(tc.mgr, tc.code, tc.out)
		var nf *pm.NotFoundError
		got := errors.As(err, &nf)
		if got != tc.wantNF {
			t.Fatalf("%s code %d %q not-found=%v want %v err=%v", tc.mgr, tc.code, tc.out, got, tc.wantNF, err)
		}
	}
}
