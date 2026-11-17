package install_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

func winTestEnv(root string, onPath ...string) install.WinEnv {
	path := map[string]bool{}
	for _, b := range onPath {
		path[b] = true
	}
	return install.WinEnv{ScoopRoot: root, Look: func(bin string) (string, bool) {
		if path[bin] {
			return `C:\elsewhere\` + bin + ".exe", true
		}
		return "", false
	}}
}

func TestBuildWindowsStepsOrderAndArgv(t *testing.T) {
	root := filepath.Join("C:", "Users", "u", "scoop")
	steps, err := install.BuildWindowsSteps([]catalog.ID{"git", "wezterm", "python"}, winTestEnv(root))
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	var kinds []install.Kind
	for _, st := range steps {
		got = append(got, st.Argv)
		kinds = append(kinds, st.Kind)
		if len(st.Argv) > 0 && st.Argv[0] == "sudo" {
			t.Fatalf("sudo on windows: %v", st.Argv)
		}
	}
	want := [][]string{
		{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force; irm get.scoop.sh | iex"},
		{"scoop", "bucket", "add", "extras"},
		{"scoop", "install", "extras/wezterm"},
		{"scoop", "install", "python"},
		{"scoop", "install", "git"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(kinds, []install.Kind{install.KindScoopBootstrap, install.KindScoopBucket, install.KindPackage, install.KindPackage, install.KindPackage}) {
		t.Fatalf("kinds %v", kinds)
	}
	if steps[4].ProbeFiles[0] != filepath.Join(root, "shims", "git.exe") {
		t.Fatalf("git probe %v", steps[4].ProbeFiles)
	}
}

func TestBuildWindowsStepsNoBucketWithoutWezterm(t *testing.T) {
	steps, err := install.BuildWindowsSteps([]catalog.ID{"git"}, winTestEnv("/r", "git"))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].Kind != install.KindScoopBootstrap || steps[1].Argv[2] != "git" {
		t.Fatalf("steps %+v", steps)
	}
	if len(steps[1].ProbeFiles) != 2 {
		t.Fatalf("PATH hit not used as probe: %v", steps[1].ProbeFiles)
	}
	if _, err := install.BuildWindowsSteps([]catalog.ID{"jq"}, winTestEnv("/r")); err == nil {
		t.Fatal("jq accepted on windows")
	}
	if steps, _ := install.BuildWindowsSteps(nil, winTestEnv("/r")); steps != nil {
		t.Fatal("empty selection has steps")
	}
}

func TestRunWindowsStepsBootstrapsScoop(t *testing.T) {
	root := "/r"
	steps, _ := install.BuildWindowsSteps([]catalog.ID{"wezterm", "git"}, winTestEnv(root))
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		if argv[0] == "scoop" && argv[1] == "bucket" {
			return "WARN  The 'extras' bucket already exists.", 2, nil
		}
		return "", 0, nil
	}}
	r := &install.Runner{Exec: ex, Home: "/h", Exists: func(string) bool { return false }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 {
		t.Fatalf("code %d results %+v", code, results)
	}
	if len(ex.calls) != 4 || ex.calls[0][0] != "powershell" {
		t.Fatalf("calls %v", ex.calls)
	}
	if len(results) != 3 || results[0].ID != install.ScoopStepID || results[0].Status != "ok" || results[1].ID != "wezterm" || results[2].ID != "git" {
		t.Fatalf("results %+v", results)
	}
}

func TestRunWindowsStepsSkipsWhatIsThere(t *testing.T) {
	root := "/r"
	steps, _ := install.BuildWindowsSteps([]catalog.ID{"wezterm", "git"}, winTestEnv(root))
	have := map[string]bool{
		filepath.Join(root, "shims", "scoop.cmd"):   true,
		filepath.Join(root, "buckets", "extras"):    true,
		filepath.Join(root, "shims", "wezterm.exe"): true,
	}
	ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }}
	r := &install.Runner{Exec: ex, Home: "/h", Exists: func(p string) bool { return have[p] }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(ex.calls) != 1 || !reflect.DeepEqual(ex.calls[0], []string{"scoop", "install", "git"}) {
		t.Fatalf("code %d calls %v", code, ex.calls)
	}
	if len(results) != 2 || results[0].Status != "satisfied" || results[1].Status != "ok" {
		t.Fatalf("results %+v", results)
	}
}

func TestRunWindowsBootstrapFailureStops(t *testing.T) {
	steps, _ := install.BuildWindowsSteps([]catalog.ID{"git"}, winTestEnv("/r"))
	ex := &fakeExec{fn: func([]string) (string, int, error) {
		return "Running the installer as administrator is disabled", 1, nil
	}}
	r := &install.Runner{Exec: ex, Home: "/h", Exists: func(string) bool { return false }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 2 || len(ex.calls) != 1 || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("code %d calls %v results %+v", code, ex.calls, results)
	}
}

func TestWindowsInstalled(t *testing.T) {
	root := "/r"
	got := install.WindowsInstalled(winTestEnv(root, "git"), func(p string) bool { return p == filepath.Join(root, "shims", "python.exe") })
	if !reflect.DeepEqual(got, map[catalog.ID]bool{"python": true, "git": true}) {
		t.Fatalf("%v", got)
	}
}
