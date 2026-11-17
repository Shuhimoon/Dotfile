package pm_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

func TestScoopArgv(t *testing.T) {
	cases := map[string][2][]string{
		"wezterm": {{"scoop", "install", "extras/wezterm"}, {"scoop", "uninstall", "wezterm"}},
		"python":  {{"scoop", "install", "python"}, {"scoop", "uninstall", "python"}},
		"git":     {{"scoop", "install", "git"}, {"scoop", "uninstall", "git"}},
	}
	for id, want := range cases {
		it, ok := pm.ScoopFor(catalog.ID(id))
		if !ok {
			t.Fatalf("%s missing", id)
		}
		if !reflect.DeepEqual(it.InstallArgv(), want[0]) || !reflect.DeepEqual(it.RemoveArgv(), want[1]) {
			t.Fatalf("%s: %v %v", id, it.InstallArgv(), it.RemoveArgv())
		}
		for _, argv := range want {
			if err := pm.ValidateArgv(argv); err != nil {
				t.Fatalf("%v: %v", argv, err)
			}
		}
	}
	if _, ok := pm.ScoopFor("jq"); ok {
		t.Fatal("jq is not in the Windows catalog")
	}
}

func TestValidateScoopArgv(t *testing.T) {
	good := [][]string{{"scoop", "bucket", "add", "extras"}, {"scoop", "install", "git"}}
	for _, a := range good {
		if err := pm.ValidateArgv(a); err != nil {
			t.Fatalf("%v: %v", a, err)
		}
	}
	bad := [][]string{
		{"sudo", "scoop", "install", "git"},
		{"scoop", "install", "git;calc"},
		{"scoop", "install", "git", "python"},
		{"scoop", "update", "*"},
		{"scoop", "bucket", "rm", "extras"},
		{"scoop", "install", "-g"},
		{"powershell", "-Command", "x"},
	}
	for _, a := range bad {
		if err := pm.ValidateArgv(a); err == nil {
			t.Fatalf("%v accepted", a)
		}
	}
}

func TestScoopBootstrapAndPaths(t *testing.T) {
	argv := pm.ScoopBootstrapArgv()
	if argv[0] != "powershell" || !strings.Contains(argv[len(argv)-1], "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser") || !strings.Contains(argv[len(argv)-1], "irm get.scoop.sh | iex") {
		t.Fatalf("bootstrap %v", argv)
	}
	if got := pm.ScoopRoot("/h", ""); got != filepath.Join("/h", "scoop") {
		t.Fatal(got)
	}
	if got := pm.ScoopRoot("/h", "/d/scoop"); got != "/d/scoop" {
		t.Fatal(got)
	}
	if got := pm.ScoopShim("/r", "git"); got != filepath.Join("/r", "shims", "git.exe") {
		t.Fatal(got)
	}
}
