package catalog_test

import (
	"slices"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
)

func TestDefaultSelectionSet(t *testing.T) {
	want := []catalog.ID{
		"git", "curl", "wget", "unzip", "rsync", "vim", "less",
		"net-tools", "whois", "dnsutils", "traceroute",
		"eza", "trash", "ripgrep", "fd", "bat", "jq", "fzf",
		"zellij", "mitos",
	}
	got := catalog.DefaultIDs()
	if !slices.Equal(got, want) {
		t.Fatalf("DefaultIDs() = %v, want %v", got, want)
	}
	if len(got) != 20 {
		t.Fatalf("len = %d, want 20", len(got))
	}

	selected := map[catalog.ID]bool{}
	for _, id := range got {
		selected[id] = true
	}
	if len(selected) != 20 {
		t.Fatalf("selection len = %d", len(selected))
	}
	for _, id := range catalog.IDs() {
		_, def := selected[id]
		inWant := slices.Contains(want, id)
		if def != inWant {
			t.Fatalf("id %s selected=%v want %v", id, def, inWant)
		}
	}

	var fromDefault []catalog.ID
	for _, c := range catalog.All() {
		for _, it := range c.Items {
			if it.Default {
				fromDefault = append(fromDefault, it.ID)
			}
		}
	}
	// DefaultIDs is built from the Default flags, in All() order.
	if !slices.Equal(got, fromDefault) {
		t.Fatalf("DefaultIDs() %v not the Default items in order %v", got, fromDefault)
	}
	sortedWant := append([]catalog.ID(nil), want...)
	slices.Sort(sortedWant)
	slices.Sort(fromDefault)
	if !slices.Equal(fromDefault, sortedWant) {
		t.Fatalf("Default flags drifted: %v vs %v", fromDefault, sortedWant)
	}
}

func TestDefaultMarkerIndependentOfChecked(t *testing.T) {
	selected := map[catalog.ID]bool{}
	for _, id := range catalog.DefaultIDs() {
		selected[id] = true
	}
	selected["git"] = false
	selected["neovim"] = true

	git, ok := catalog.ItemByID("git")
	if !ok || !git.Default {
		t.Fatalf("git Default = %v, %v; want true after uncheck", git.Default, ok)
	}
	nvim, ok := catalog.ItemByID("neovim")
	if !ok || nvim.Default {
		t.Fatalf("neovim Default = %v, %v; want false after check", nvim.Default, ok)
	}
	if selected["git"] {
		t.Fatal("git still checked")
	}
	if !selected["neovim"] {
		t.Fatal("neovim not checked")
	}
}

func TestCatalogIDsUnique(t *testing.T) {
	ids := catalog.IDs()
	if len(ids) != 35 {
		t.Fatalf("len = %d, want 35", len(ids))
	}
	seen := map[catalog.ID]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate %s", id)
		}
		seen[id] = true
	}
}

func TestCategoryOrder(t *testing.T) {
	want := []struct {
		id, title string
		items     []string
	}{
		{"basics", "基礎", []string{"git", "curl", "wget", "unzip", "rsync", "vim", "less", "net-tools", "whois", "dnsutils", "traceroute"}},
		{"shell", "Shell", []string{"zsh", "starship", "fastfetch"}},
		{"files", "檔案搜尋", []string{"eza", "trash", "ripgrep", "fd", "bat", "jq", "fzf"}},
		{"editors", "編輯器", []string{"neovim", "zellij", "mitos"}},
		{"languages", "語言", []string{"go", "rust", "python", "node", "zig"}},
		{"build", "編譯", []string{"make", "cmake", "compiler", "pkg-config"}},
		{"containers", "容器", []string{"docker", "podman"}},
	}
	got := catalog.All()
	if len(got) != len(want) {
		t.Fatalf("categories = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].ID != w.id || got[i].Title != w.title {
			t.Fatalf("category %d = %s/%s, want %s/%s", i, got[i].ID, got[i].Title, w.id, w.title)
		}
		var names []string
		for _, it := range got[i].Items {
			names = append(names, it.Name)
			if it.Name != string(it.ID) {
				t.Fatalf("name %s != id %s", it.Name, it.ID)
			}
		}
		if !slices.Equal(names, w.items) {
			t.Fatalf("category %s items = %v, want %v", w.id, names, w.items)
		}
	}
}
