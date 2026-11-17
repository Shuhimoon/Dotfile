package tui

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
)

// Regression: on a machine where a category is fully installed, a and A
// used to skip every installed row and so did nothing at all.
func TestCatAllTogglesWhenCategoryInstalled(t *testing.T) {
	m := New(fedoraInfo())
	installed := map[catalog.ID]bool{}
	for _, id := range m.idsInCategory("basics") {
		installed[id] = true
	}
	installed["eza"] = true
	m.applyInstalled(installed)
	m.cursor = 0 // git, basics
	m = press(t, m, keyRune('a'))
	for _, id := range m.idsInCategory("basics") {
		if !m.selected[id] {
			t.Fatalf("a did not tick installed %s", id)
		}
	}
	if m.status == "" {
		t.Fatal("no hint that ticked installed tools are downloaded again")
	}
	m.width, m.height = 120, 60
	if v := render(m, false); !strings.Contains(v, "[x] git") || !strings.Contains(v, "重新下載") {
		t.Fatalf("ticks not visible\n%s", v)
	}
	m = press(t, m, keyRune('a'))
	for _, id := range m.idsInCategory("basics") {
		if m.selected[id] {
			t.Fatalf("second a left %s ticked", id)
		}
	}
}

func TestCatAllSkipsInstalledInMixedCategory(t *testing.T) {
	m := New(fedoraInfo())
	m.applyInstalled(map[catalog.ID]bool{"eza": true})
	for i, id := range m.flat {
		if id == "trash" {
			m.cursor = i
		}
	}
	m = press(t, m, keyRune('a')) // files default all on (minus installed eza) -> clear
	for _, id := range m.idsInCategory("files") {
		if m.selected[id] {
			t.Fatalf("%s still ticked", id)
		}
	}
	m = press(t, m, keyRune('a'))
	for _, id := range m.idsInCategory("files") {
		if want := id != "eza"; m.selected[id] != want {
			t.Fatalf("%s ticked=%v", id, m.selected[id])
		}
	}
}

func TestAllTogglesEverything(t *testing.T) {
	m := New(fedoraInfo())
	m = press(t, m, keyRune('A'))
	if n := m.countSelected(); n != len(m.flat) {
		t.Fatalf("A ticked %d of %d", n, len(m.flat))
	}
	m = press(t, m, keyRune('A'))
	if n := m.countSelected(); n != 0 {
		t.Fatalf("second A left %d", n)
	}
	// Everything installed: A still toggles, as re-downloads.
	all := map[catalog.ID]bool{}
	for _, id := range m.flat {
		all[id] = true
	}
	m.applyInstalled(all)
	m = press(t, m, keyRune('A'))
	if n := m.countSelected(); n != len(m.flat) || m.status == "" {
		t.Fatalf("A on all-installed ticked %d, status %q", n, m.status)
	}
}

func TestUninstallCatAllAndAll(t *testing.T) {
	m := uninstallFixture(t, "git", "jq", "fzf", "starship", "zsh", "python")
	ids := m.removableIDs()
	m.ucursor = 0 // first removable: a Shell item
	cat := m.categoryOf(ids[0])
	m = press(t, m, keyRune('a'))
	for _, id := range ids {
		if want := m.categoryOf(id) == cat; m.uselected[id] != want {
			t.Fatalf("a: %s ticked=%v", id, m.uselected[id])
		}
	}
	m = press(t, m, keyRune('A'))
	for _, id := range ids {
		if !m.uselected[id] {
			t.Fatalf("A: %s not ticked", id)
		}
	}
	m = press(t, m, keyRune('A'))
	for _, id := range ids {
		if m.uselected[id] {
			t.Fatalf("second A: %s still ticked", id)
		}
	}
}
