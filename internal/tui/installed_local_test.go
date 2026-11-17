package tui

import (
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

// Tools installed from a GitHub release, the Go tarball, or rustup live outside
// the package paths and must still be marked installed.
func TestMarksLocalInstalls(t *testing.T) {
	dnf, err := install.ResolveAll(catalog.IDs(), "dnf")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{
		"/h/.local/bin/zellij":   true,
		"/h/.local/bin/starship": true,
		"/h/.local/bin/ms":       true,
		"/usr/local/go/bin/go":   true,
		"/h/.cargo/bin/rustc":    true,
	}
	exists := func(p string) bool { return files[p] }
	got := marksFromPlans(dnf, "/h", exists, false)
	for _, id := range []catalog.ID{"zellij", "starship", "mitos", "go"} {
		if !got[id] {
			t.Fatalf("%s not marked: %v", id, got)
		}
	}
	if got["rust"] || len(got) != 4 {
		t.Fatalf("rustc alone or extra marks: %v", got)
	}
	files["/h/.cargo/bin/cargo"] = true
	if !marksFromPlans(dnf, "/h", exists, false)["rust"] {
		t.Fatal("rustup install not marked")
	}
	if got := marksFromPlans(dnf, "", exists, false); got["zellij"] || got["rust"] || !got["go"] {
		t.Fatalf("empty home must skip home probes: %v", got)
	}
}
