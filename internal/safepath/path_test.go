package safepath_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/safepath"
)

func TestRejectEscapingParentSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "link", "config.kdl")
	err := safepath.RejectEscapingSymlink(path, root)
	if err == nil || !strings.Contains(err.Error(), "拒絕 symlink：") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectEscapingOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := safepath.RejectEscapingSymlink("/etc/passwd", root); err == nil {
		t.Fatal("expected /etc/passwd to be rejected")
	}
	outside := filepath.Join(root, "..", "passwd")
	if err := safepath.RejectEscapingSymlink(outside, root); err == nil {
		t.Fatal("expected ../ path to be rejected")
	}
	ok := filepath.Join(root, "inside")
	if err := os.WriteFile(ok, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := safepath.RejectEscapingSymlink(ok, root); err != nil {
		t.Fatal(err)
	}
	if err := safepath.RejectEscapingSymlink(filepath.Join(root, "missing"), root); err != nil {
		t.Fatal(err)
	}
}
