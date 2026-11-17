package dotfiles

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/shuhi/dotfiles/internal/configcopy"
)

// TestEmbeddedConfigsCoverFilesFor: every file FilesFor would deploy from
// the repo also exists in the embedded copy used by released binaries.
func TestEmbeddedConfigsCoverFilesFor(t *testing.T) {
	dir := t.TempDir()
	if err := configcopy.Extract(Configs, dir); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	for _, id := range configcopy.ToolIDs {
		fromRepo, err := configcopy.FilesFor(id, ".", home, "")
		if err != nil || len(fromRepo) == 0 {
			t.Fatalf("%s repo: %v", id, err)
		}
		fromEmbed, err := configcopy.FilesFor(id, dir, home, "")
		if err != nil || len(fromEmbed) != len(fromRepo) {
			t.Fatalf("%s embed: %d vs %d files (%v)", id, len(fromEmbed), len(fromRepo), err)
		}
		for i := range fromRepo {
			rel, _ := filepath.Rel(".", fromRepo[i].Src)
			erel, _ := filepath.Rel(dir, fromEmbed[i].Src)
			if rel != erel || fromRepo[i].Dst != fromEmbed[i].Dst {
				t.Fatalf("%s: %s vs %s", id, rel, erel)
			}
			if _, err := fs.Stat(Configs, filepath.ToSlash(rel)); err != nil {
				t.Fatalf("%s not embedded: %v", rel, err)
			}
		}
	}
}
