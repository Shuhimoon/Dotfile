package configcopy

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestExtractWritesAndRefreshes(t *testing.T) {
	fsys := fstest.MapFS{
		"starship/starship.toml":       {Data: []byte("theme")},
		"zellij/plugins/zjstatus.wasm": {Data: []byte{0, 1, 2}},
		"shells/zsh/zshrc":             {Data: []byte("zshrc")},
	}
	dir := t.TempDir()
	if err := Extract(fsys, dir); err != nil {
		t.Fatal(err)
	}
	for p, f := range fsys {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil || string(got) != string(f.Data) {
			t.Fatalf("%s: %q %v", p, got, err)
		}
	}
	// Unchanged files are not rewritten; changed ones are.
	keep := filepath.Join(dir, "shells", "zsh", "zshrc")
	old := time.Unix(1000, 0)
	if err := os.Chtimes(keep, old, old); err != nil {
		t.Fatal(err)
	}
	fsys["starship/starship.toml"] = &fstest.MapFile{Data: []byte("theme v2")}
	if err := Extract(fsys, dir); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(keep); !fi.ModTime().Equal(old) {
		t.Fatal("unchanged file was rewritten")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "starship", "starship.toml")); string(got) != "theme v2" {
		t.Fatalf("not refreshed: %q", got)
	}
	if err := Extract(fsys, "relative"); err == nil {
		t.Fatal("relative dir accepted")
	}
}
