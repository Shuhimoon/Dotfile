package configcopy

import (
	"io/fs"
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

// dotDotFS lists an extra ".." entry at its root, which fs.ValidPath rejects.
type dotDotFS struct{ fstest.MapFS }

func (f dotDotFS) ReadDir(name string) ([]fs.DirEntry, error) {
	ents, err := f.MapFS.ReadDir(name)
	if name == "." {
		ents = append([]fs.DirEntry{dotDotEntry{}}, ents...)
	}
	return ents, err
}

type dotDotEntry struct{}

func (dotDotEntry) Name() string               { return ".." }
func (dotDotEntry) IsDir() bool                { return false }
func (dotDotEntry) Type() fs.FileMode          { return 0 }
func (dotDotEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

func TestExtractRejectsInvalidPath(t *testing.T) {
	dir := t.TempDir()
	err := Extract(dotDotFS{fstest.MapFS{"a": {Data: []byte("a")}}}, dir)
	if err == nil || err.Error() != "extract: 不合法的路徑 .." {
		t.Fatalf("err %v", err)
	}
	// ".." inside a name is a valid path, not a parent reference.
	if err := Extract(fstest.MapFS{"zellij/a..b.kdl": {Data: []byte("x")}}, dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "zellij", "a..b.kdl")); string(got) != "x" {
		t.Fatalf("got %q", got)
	}
}
