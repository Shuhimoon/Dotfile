package install_test

import (
	"testing"

	"github.com/shuhi/dotfiles/internal/install"
)

func TestDebianFdBatSymlink(t *testing.T) {
	stat := func(p string) bool {
		return p == "/usr/bin/fdfind" || p == "/usr/bin/batcat"
	}
	got := install.AliasSymlinks("/home/u", stat)
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got[0].Link != "/home/u/.local/bin/fd" || got[0].Target != "/usr/bin/fdfind" {
		t.Fatalf("fd %+v", got[0])
	}
	if got[1].Link != "/home/u/.local/bin/bat" || got[1].Target != "/usr/bin/batcat" {
		t.Fatalf("bat %+v", got[1])
	}
}

func TestFedoraNoAlias(t *testing.T) {
	// 路徑存在不是 sharkdp 的身分證明：bacula-console-bat 也提供 /usr/bin/bat。
	stat := func(p string) bool {
		return p == "/usr/bin/fd" || p == "/usr/bin/bat"
	}
	got := install.AliasSymlinks("/home/user", stat)
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
