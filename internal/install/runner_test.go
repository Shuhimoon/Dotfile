package install_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

type fakeExec struct {
	fn    func(argv []string) (string, int, error)
	calls [][]string
}

func (f *fakeExec) Run(ctx context.Context, argv []string) (string, int, error) {
	f.calls = append(f.calls, append([]string(nil), argv...))
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if f.fn == nil {
		return "", 0, nil
	}
	return f.fn(argv)
}

func TestUpdateFailureAbortsBeforeItems(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"git", "curl"}, "apt")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		if len(argv) > 0 && argv[len(argv)-1] == "update" {
			return "fail", 1, nil
		}
		t.Fatalf("install argv ran: %v", argv)
		return "", 0, nil
	}}
	logged := false
	r := &install.Runner{Exec: ex, EUID: 1000, Log: func([]install.ItemResult) error {
		logged = true
		return nil
	}}
	_, code := r.RunSteps(context.Background(), steps)
	if code != 2 {
		t.Fatalf("code %d", code)
	}
	if logged {
		t.Fatal("update failure must not log")
	}
	if len(ex.calls) != 1 {
		t.Fatalf("calls %v", ex.calls)
	}
}

func TestItemFailureDoesNotStopLaterItems(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"git", "curl"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 0, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		n++
		if n == 1 {
			return "bad", 1, nil
		}
		return "", 0, nil
	}}
	var logged []install.ItemResult
	r := &install.Runner{Exec: ex, EUID: 0, Exists: func(string) bool { return false }, Log: func(res []install.ItemResult) error {
		logged = res
		return nil
	}}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 2 {
		t.Fatalf("code %d results %d", code, len(results))
	}
	if results[0].Status != "failed" || results[1].Status != "ok" {
		t.Fatalf("%+v", results)
	}
	if len(ex.calls) != 2 || logged == nil {
		t.Fatalf("calls %d logged %v", len(ex.calls), logged)
	}
}

func TestFallbackOnlyOnNotFound(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"starship"}, "apt")
	if err != nil {
		t.Fatal(err)
	}
	base, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(out string, code int, runErr error) (called bool, results []install.ItemResult) {
		steps := append([]install.Step(nil), base...)
		// copy fallback so Local mutation does not leak
		for i := range steps {
			if steps[i].Fallback != nil {
				fb := *steps[i].Fallback
				steps[i].Fallback = &fb
			}
		}
		var hit bool
		for i := range steps {
			if steps[i].Fallback == nil {
				continue
			}
			steps[i].Fallback.Local = func(context.Context) error {
				hit = true
				return nil
			}
		}
		ex := &fakeExec{fn: func(argv []string) (string, int, error) {
			if strings.Contains(strings.Join(argv, " "), "update") {
				return "", 0, nil
			}
			return out, code, runErr
		}}
		r := &install.Runner{Exec: ex, EUID: 1000, Exists: func(string) bool { return false }}
		results, _ = r.RunSteps(context.Background(), steps)
		return hit, results
	}
	called, results := run("E: Unable to locate package starship", 100, nil)
	if !called {
		t.Fatal("not-found should run fallback")
	}
	if len(results) != 1 || results[0].Status != "ok" || results[0].Method != "release" || !results[0].NeedsLocalBin {
		t.Fatalf("%+v", results)
	}
	called, results = run("other", 2, nil)
	if called || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("other non-zero called %v %+v", called, results)
	}
	called, results = run("", 0, fmt.Errorf("fork"))
	if called || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("start err called %v %+v", called, results)
	}
}

func TestNonZeroExitIsNotAnError(t *testing.T) {
	_, code, err := (install.OSExecutor{}).Run(context.Background(), []string{"/bin/false"})
	if err != nil {
		t.Fatalf("non-zero exit must be err=nil, got %v", err)
	}
	if code == 0 {
		t.Fatal("expected non-zero exitCode")
	}
	_, _, err = (install.OSExecutor{}).Run(context.Background(), []string{"/nonexistent/dotfiles-tools-missing"})
	if err == nil {
		t.Fatal("start failure must be err")
	}
	_, _, err = (install.OSExecutor{}).Run(context.Background(), []string{"/bin/bash", "-c", "exit 130"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("exit 130 must cancel, got %v", err)
	}
}

func TestExit130StopsLaterSteps(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"git", "zellij"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	steps, err := install.BuildSteps(plans, 0, home)
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		return "interrupted", 130, nil
	}}
	releaseCalled := false
	applyCalled := false
	logCalls := 0
	r := &install.Runner{
		Exec:   ex,
		EUID:   0,
		Home:   home,
		Exists: func(string) bool { return false },
		Release: func(context.Context, string, string, release.AssetQuery) error {
			releaseCalled = true
			return nil
		},
		Rc: func(string, string, shellrc.Shell, []shellrc.BlockID) ([]shellrc.BlockID, error) {
			applyCalled = true
			return nil, nil
		},
		Log: func([]install.ItemResult) error {
			logCalls++
			return nil
		},
	}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 0 || releaseCalled || applyCalled || logCalls != 1 || len(ex.calls) != 1 {
		t.Fatalf("code %d results %d release %v apply %v log %d calls %d", code, len(results), releaseCalled, applyCalled, logCalls, len(ex.calls))
	}
}

func TestCancelStopsLaterSteps(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"git", "curl", "wget"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 0, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	ex := &fakeExec{fn: func(argv []string) (string, int, error) {
		n++
		if n == 2 {
			return "", 0, context.Canceled
		}
		return "", 0, nil
	}}
	var logged []install.ItemResult
	logCalls := 0
	r := &install.Runner{Exec: ex, EUID: 0, Exists: func(string) bool { return false }, Log: func(res []install.ItemResult) error {
		logCalls++
		logged = append([]install.ItemResult(nil), res...)
		return nil
	}}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 {
		t.Fatalf("code %d", code)
	}
	if n != 2 {
		t.Fatalf("calls %d", n)
	}
	if len(results) != 1 || results[0].ID != "git" {
		t.Fatalf("canceled step appended: %+v", results)
	}
	if logCalls != 1 || len(logged) != 1 {
		t.Fatalf("log %d %+v", logCalls, logged)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	plans, err := install.ResolveAll(catalog.DefaultIDs(), "dnf")
	if err != nil {
		t.Fatal(err)
	}
	r := &install.Runner{
		DryRun: true,
		EUID:   1000,
		Home:   t.TempDir(),
		Exec: &fakeExec{fn: func(argv []string) (string, int, error) {
			t.Fatalf("exec %v", argv)
			return "", 0, nil
		}},
		Log: func([]install.ItemResult) error {
			t.Fatal("log")
			return nil
		},
	}
	_, code := r.Run(context.Background(), plans)
	if code != 0 {
		t.Fatalf("code %d", code)
	}
}

func TestAlreadySatisfiedUsesProbeNotPackageQuery(t *testing.T) {
	exists := func(path string) bool {
		return path == "/usr/bin/whois" || path == "/sbin/traceroute" || path == "/usr/bin/git"
	}
	if !install.AlreadySatisfied(pm.MethodPackage, []string{"/usr/bin/whois"}, nil, exists, false) {
		t.Fatal("whois probe")
	}
	if !install.AlreadySatisfied(pm.MethodPackage, []string{"/usr/sbin/traceroute", "/sbin/traceroute"}, nil, exists, false) {
		t.Fatal("traceroute OR")
	}
	if install.AlreadySatisfied(pm.MethodPackage, nil, nil, exists, false) {
		t.Fatal("empty probes")
	}
	if !install.AlreadySatisfied(pm.MethodPreinstalled, []string{"/usr/bin/make"}, nil, func(string) bool { return true }, false) {
		t.Fatal("make")
	}
	if install.AlreadySatisfied(pm.MethodPreinstalled, []string{"/sbin/ifconfig"}, nil, func(path string) bool {
		return path == "/usr/sbin/ifconfig"
	}, false) {
		t.Fatal("/usr/sbin/ifconfig is not net-tools")
	}
	if !install.AlreadySatisfied(pm.MethodXcodeCLT, nil, nil, exists, true) {
		t.Fatal("xcode")
	}
	if install.AlreadySatisfied(pm.MethodXcodeCLT, nil, nil, exists, false) {
		t.Fatal("xcode missing")
	}
	for _, method := range []pm.Method{pm.MethodRelease, pm.MethodGoTarball, pm.MethodRustup} {
		if install.AlreadySatisfied(method, []string{"/usr/local/go"}, nil, exists, true) {
			t.Fatalf("%s", method)
		}
	}
	groups := [][]string{{"/usr/bin/gcc"}, {"/usr/bin/g++"}}
	gccOnly := func(path string) bool { return path == "/usr/bin/gcc" }
	if install.AlreadySatisfied(pm.MethodPackage, []string{"/usr/bin/gcc", "/usr/bin/g++"}, groups, gccOnly, false) {
		t.Fatal("one compiler group is not enough")
	}
	both := func(path string) bool { return path == "/usr/bin/gcc" || path == "/usr/bin/g++" }
	if !install.AlreadySatisfied(pm.MethodPackage, nil, groups, both, false) {
		t.Fatal("groups ignore an empty flat list")
	}
	if install.AlreadySatisfied(pm.MethodPackage, []string{"/usr/bin/gcc"}, groups, gccOnly, false) {
		t.Fatal("flat probe must not override groups")
	}
}

func TestXcodeSelectPresentIgnoresStdout(t *testing.T) {
	const dev = "/Library/Developer/CommandLineTools"
	cls := install.ClassExec("", dev+"\n", 0, nil)
	if cls.ErrText != "" || !install.XcodeSelectPresent(cls) {
		t.Fatalf("present %+v", cls)
	}
	stuffed := cls
	stuffed.ErrText = dev
	if install.XcodeSelectPresent(stuffed) {
		t.Fatal("stdout passed as ErrText must not count as present")
	}
	if install.XcodeSelectPresent(install.ExecClass{Canceled: true, ExitCode: 0}) {
		t.Fatal("canceled")
	}
	if install.XcodeSelectPresent(install.ExecClass{ExecErr: errors.New("missing"), ErrText: "missing"}) {
		t.Fatal("exec err")
	}
	if install.XcodeSelectPresent(install.ExecClass{ExitCode: 2, ErrText: "命令失敗 exit 2"}) {
		t.Fatal("exit 2")
	}
}

func countArgv(calls [][]string, last string) int {
	n := 0
	for _, c := range calls {
		if len(c) > 0 && c[len(c)-1] == last {
			n++
		}
	}
	return n
}

func xcodeRunner(t *testing.T, force bool, fn func(argv []string) (string, int, error)) (*install.Runner, *fakeExec, []install.Step) {
	t.Helper()
	plans, err := install.ResolveAll([]catalog.ID{"compiler"}, "brew")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: fn}
	r := &install.Runner{Exec: ex, EUID: 1000, Force: force, Home: t.TempDir()}
	return r, ex, steps
}

func TestRunXcodeStdoutDoesNotInstallUnlessForced(t *testing.T) {
	const dev = "/Library/Developer/CommandLineTools\n"
	fn := func(argv []string) (string, int, error) {
		if len(argv) == 2 && argv[0] == "xcode-select" && argv[1] == "-p" {
			return dev, 0, nil
		}
		if len(argv) == 2 && argv[1] == "--install" {
			return "", 0, nil
		}
		return "", 1, fmt.Errorf("unexpected %v", argv)
	}
	r, ex, steps := xcodeRunner(t, false, fn)
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "satisfied" || results[0].ErrText != "" {
		t.Fatalf("code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 0 || countArgv(ex.calls, "-p") != 1 {
		t.Fatalf("calls %v", ex.calls)
	}

	r, ex, steps = xcodeRunner(t, true, fn)
	results, code = r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "satisfied" {
		t.Fatalf("force code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 1 || countArgv(ex.calls, "-p") != 2 {
		t.Fatalf("force calls %v", ex.calls)
	}
}

func TestRunXcodeExecErrDoesNotInstall(t *testing.T) {
	r, ex, steps := xcodeRunner(t, true, func(argv []string) (string, int, error) {
		if len(argv) == 2 && argv[1] == "-p" {
			return "", 0, errors.New("xcode-select: not found")
		}
		t.Fatalf("fell through %v", argv)
		return "", 0, nil
	})
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 1 || results[0].Status != "failed" || results[0].ErrText == "" {
		t.Fatalf("code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 0 {
		t.Fatalf("calls %v", ex.calls)
	}
}

func TestRunXcodeCancelDoesNotInstall(t *testing.T) {
	r, ex, steps := xcodeRunner(t, true, func(argv []string) (string, int, error) {
		return "", 0, context.Canceled
	})
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 0 || countArgv(ex.calls, "--install") != 0 || len(ex.calls) != 1 {
		t.Fatalf("code %d results %+v calls %v", code, results, ex.calls)
	}
}

func TestRunXcodeInstallNonZeroStillProbes(t *testing.T) {
	const dev = "/Library/Developer/CommandLineTools\n"
	fn := func(argv []string) (string, int, error) {
		switch argv[len(argv)-1] {
		case "-p":
			return dev, 0, nil
		case "--install":
			return "xcode-select: note: Command Line Tools are already installed", 1, nil
		default:
			return "", 1, fmt.Errorf("unexpected %v", argv)
		}
	}
	r, ex, steps := xcodeRunner(t, true, fn)
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "satisfied" || results[0].ErrText != "" {
		t.Fatalf("code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 1 || countArgv(ex.calls, "-p") != 2 {
		t.Fatalf("calls %v", ex.calls)
	}

	probes := 0
	r, ex, steps = xcodeRunner(t, false, func(argv []string) (string, int, error) {
		switch argv[len(argv)-1] {
		case "-p":
			probes++
			if probes == 1 {
				return "xcode-select: error", 2, nil
			}
			return dev, 0, nil
		case "--install":
			return "", 0, errors.New("xcode-select: not found")
		default:
			return "", 1, fmt.Errorf("unexpected %v", argv)
		}
	})
	results, code = r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 1 || results[0].Status != "failed" || results[0].ErrText == "" {
		t.Fatalf("start code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 1 || countArgv(ex.calls, "-p") != 1 {
		t.Fatalf("start calls %v", ex.calls)
	}
}

func TestRunXcodeSecondProbePending(t *testing.T) {
	n := 0
	r, ex, steps := xcodeRunner(t, true, func(argv []string) (string, int, error) {
		if argv[len(argv)-1] == "-p" {
			n++
			if n == 1 {
				return "/Library/Developer/CommandLineTools\n", 0, nil
			}
			return "xcode-select: error", 2, nil
		}
		return "xcode-select: note: already installed", 1, nil
	})
	results, code := r.RunSteps(context.Background(), steps)
	if code != 1 || len(results) != 1 || results[0].Status != "pending-user" || results[0].ErrText != pm.XcodePendingMessage {
		t.Fatalf("code %d %+v", code, results)
	}
	if countArgv(ex.calls, "--install") != 1 {
		t.Fatalf("calls %v", ex.calls)
	}
}

func TestPackageProbeSkipAndForce(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"whois"}, "brew")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exists := func(path string) bool { return path == "/usr/bin/whois" }
	run := func(force bool) ([]install.ItemResult, [][]string) {
		t.Helper()
		ex := &fakeExec{fn: func(argv []string) (string, int, error) {
			return "", 0, nil
		}}
		var logged []install.ItemResult
		r := &install.Runner{
			Exec: ex, EUID: 1000, Force: force, Exists: exists,
			Log: func(res []install.ItemResult) error {
				logged = res
				return nil
			},
		}
		results, code := r.RunSteps(context.Background(), steps)
		if code != 0 || len(results) != 1 || len(logged) != 1 || logged[0].Status != results[0].Status || logged[0].Argv != nil && results[0].Argv == nil {
			t.Fatalf("force %v code %d %+v log %+v", force, code, results, logged)
		}
		return results, ex.calls
	}
	skipped, calls := run(false)
	if skipped[0].Status != "satisfied" || skipped[0].Argv != nil || skipped[0].Note != "" || len(calls) != 0 {
		t.Fatalf("skip %+v calls %v", skipped, calls)
	}
	forced, calls := run(true)
	if forced[0].Status != "ok" || forced[0].Note != pm.KegOnlyGenericNote || len(calls) != 1 || !slices.Equal(calls[0], []string{"brew", "install", "whois"}) {
		t.Fatalf("force %+v calls %v", forced, calls)
	}
}

func TestFzfProbeNoteWithoutArgv(t *testing.T) {
	steps := []install.Step{{
		ItemID: "fzf", Title: "install fzf", Kind: install.KindPackage,
		Method: pm.MethodPackage, Manager: "brew",
		Argv:       []string{"brew", "install", "fzf"},
		ProbeFiles: []string{"/usr/bin/fzf"},
	}}
	ex := &fakeExec{}
	r := &install.Runner{Exec: ex, Exists: func(string) bool { return true }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "satisfied" || results[0].Note != "fzf 0.48.0 以上" || results[0].Argv != nil || len(ex.calls) != 0 {
		t.Fatalf("%+v calls %v", results, ex.calls)
	}
}

func TestPreinstalledIgnoresForce(t *testing.T) {
	runID := func(id catalog.ID, exists func(string) bool) install.ItemResult {
		t.Helper()
		plans, err := install.ResolveAll([]catalog.ID{id}, "brew")
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildSteps(plans, 1000, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		ex := &fakeExec{fn: func(argv []string) (string, int, error) {
			t.Fatalf("argv %v", argv)
			return "", 0, nil
		}}
		r := &install.Runner{Exec: ex, Force: true, Exists: exists}
		results, _ := r.RunSteps(context.Background(), steps)
		if len(ex.calls) != 0 || len(results) != 1 {
			t.Fatalf("calls %v results %+v", ex.calls, results)
		}
		return results[0]
	}
	hit := runID("make", func(path string) bool { return path == "/usr/bin/make" })
	if hit.Status != "satisfied" || hit.ErrText != "" {
		t.Fatalf("hit %+v", hit)
	}
	miss := runID("make", func(string) bool { return false })
	if miss.Status != "failed" || miss.ErrText != pm.MakeMissingMessage {
		t.Fatalf("miss %+v", miss)
	}
	other := runID("net-tools", func(path string) bool { return path == "/usr/sbin/ifconfig" })
	if other.Status != "failed" || other.ErrText != pm.NetToolsMissingMessage {
		t.Fatalf("net-tools %+v", other)
	}
}

func TestTrashForceStaysShadowed(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"trash"}, "brew")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: func(argv []string) (string, int, error) { return "", 0, nil }}
	r := &install.Runner{Exec: ex, EUID: 1000, Force: true, Exists: func(string) bool { return true }}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "shadowed" || results[0].Note != pm.TrashShadowNote {
		t.Fatalf("code %d %+v", code, results)
	}
	if len(ex.calls) != 1 || !slices.Equal(ex.calls[0], []string{"brew", "install", "trash-cli"}) {
		t.Fatalf("calls %v", ex.calls)
	}
}

func TestNoProbeFilesStillInstall(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"git", "whois"}, "pacman")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 0, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(force bool) ([]install.ItemResult, [][]string) {
		t.Helper()
		ex := &fakeExec{fn: func(argv []string) (string, int, error) { return "", 0, nil }}
		r := &install.Runner{Exec: ex, EUID: 0, Force: force, Exists: func(string) bool { return true }}
		results, code := r.RunSteps(context.Background(), steps)
		if code != 0 || len(results) != 2 {
			t.Fatalf("force %v code %d %+v", force, code, results)
		}
		return results, ex.calls
	}
	skipped, calls := run(false)
	if skipped[0].Status != "satisfied" || skipped[1].Status != "satisfied" || skipped[0].Argv != nil || skipped[1].Argv != nil {
		t.Fatalf("skip %+v", skipped)
	}
	if len(calls) != 1 || !slices.Equal(calls[0], []string{"pacman", "-Sy", "--noconfirm"}) {
		t.Fatalf("skip calls %v", calls)
	}
	forced, calls := run(true)
	if forced[0].Status != "ok" || forced[1].Status != "ok" {
		t.Fatalf("force %+v", forced)
	}
	var updates, installs int
	var sawNeeded bool
	for _, c := range calls {
		if slices.Equal(c, []string{"pacman", "-Sy", "--noconfirm"}) {
			updates++
		}
		if slices.Contains(c, "-S") {
			installs++
		}
		if slices.Contains(c, "--needed") && slices.Contains(c, "whois") {
			sawNeeded = true
		}
	}
	if updates != 1 || installs != 2 || !sawNeeded {
		t.Fatalf("force calls %v", calls)
	}
}

func TestDnfBatCompletionProbe(t *testing.T) {
	const comp = "/usr/share/bash-completion/completions/bat.bash"
	run := func(mgr string, exists func(string) bool) ([]install.ItemResult, [][]string) {
		t.Helper()
		plans, err := install.ResolveAll([]catalog.ID{"bat"}, mgr)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildSteps(plans, 0, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }}
		r := &install.Runner{Exec: ex, EUID: 0, Force: false, Exists: exists}
		results, code := r.RunSteps(context.Background(), steps)
		if code != 0 || len(results) != 1 {
			t.Fatalf("%s code %d %+v", mgr, code, results)
		}
		return results, ex.calls
	}
	res, calls := run("dnf", func(p string) bool { return p == "/usr/bin/bat" || p == "/usr/sbin/bat" })
	if res[0].Status != "ok" || len(calls) != 1 || !slices.Equal(calls[0], []string{"dnf", "install", "-y", "bat"}) {
		t.Fatalf("dnf bin %+v %v", res, calls)
	}
	res, calls = run("dnf", func(p string) bool { return p == comp })
	if res[0].Status != "satisfied" || res[0].Argv != nil || len(calls) != 0 {
		t.Fatalf("dnf comp %+v %v", res, calls)
	}
	res, calls = run("yum", func(p string) bool { return p == "/usr/bin/bat" })
	if res[0].Status != "ok" || len(calls) != 1 || !slices.Equal(calls[0], []string{"yum", "install", "-y", "bat"}) {
		t.Fatalf("yum bin %+v %v", res, calls)
	}
	res, calls = run("yum", func(p string) bool { return p == comp })
	if res[0].Status != "satisfied" || res[0].Argv != nil || len(calls) != 0 {
		t.Fatalf("yum comp %+v %v", res, calls)
	}
}

func TestBrewTrashIgnoresSystemTrash(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"trash"}, "brew")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }}
	r := &install.Runner{
		Exec: ex, EUID: 1000, Force: false,
		Exists: func(path string) bool { return path == "/usr/bin/trash" },
	}
	results, code := r.RunSteps(context.Background(), steps)
	if code != 0 || len(results) != 1 || results[0].Status != "shadowed" || results[0].Note != pm.TrashShadowNote {
		t.Fatalf("code %d %+v", code, results)
	}
	if len(ex.calls) != 1 || !slices.Equal(ex.calls[0], []string{"brew", "install", "trash-cli"}) {
		t.Fatalf("calls %v", ex.calls)
	}
}

func TestAptAliasOnSatisfiedShortCircuit(t *testing.T) {
	run := func(id catalog.ID, hit string, linkName, target string, also []string) (install.ItemResult, [][]string, string) {
		t.Helper()
		home := t.TempDir()
		plans, err := install.ResolveAll([]catalog.ID{id}, "apt")
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildSteps(plans, 0, home)
		if err != nil {
			t.Fatal(err)
		}
		ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }}
		r := &install.Runner{
			Exec: ex, EUID: 0, Home: home, Force: false,
			Exists: func(path string) bool {
				if path == hit {
					return true
				}
				return slices.Contains(also, path)
			},
		}
		results, code := r.RunSteps(context.Background(), steps)
		if code != 0 || len(results) != 1 {
			t.Fatalf("%s code %d %+v", id, code, results)
		}
		return results[0], ex.calls, home
	}
	checkUpdate := func(calls [][]string) {
		t.Helper()
		if len(calls) != 1 || !slices.Equal(calls[0], []string{"apt-get", "update"}) {
			t.Fatalf("calls %v", calls)
		}
	}
	res, calls, home := run("fd", "/usr/bin/fdfind", "fd", "/usr/bin/fdfind", nil)
	if res.Status != "satisfied" || res.Argv != nil || !res.NeedsLocalBin {
		t.Fatalf("fd %+v", res)
	}
	checkUpdate(calls)
	got, err := os.Readlink(filepath.Join(home, ".local", "bin", "fd"))
	if err != nil || got != "/usr/bin/fdfind" {
		t.Fatalf("fd link %q %v", got, err)
	}
	res, calls, home = run("fd", "/usr/bin/fdfind", "fd", "/usr/bin/fdfind", []string{"/usr/bin/fd"})
	if res.Status != "satisfied" || res.Argv != nil || res.NeedsLocalBin {
		t.Fatalf("fd both %+v", res)
	}
	checkUpdate(calls)
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "fd")); !os.IsNotExist(err) {
		t.Fatalf("fd symlink exists %v", err)
	}
	res, calls, home = run("bat", "/usr/bin/batcat", "bat", "/usr/bin/batcat", nil)
	if res.Status != "satisfied" || res.Argv != nil || !res.NeedsLocalBin {
		t.Fatalf("bat %+v", res)
	}
	checkUpdate(calls)
	got, err = os.Readlink(filepath.Join(home, ".local", "bin", "bat"))
	if err != nil || got != "/usr/bin/batcat" {
		t.Fatalf("bat link %q %v", got, err)
	}
}

func TestCompilerGroupStillInstalls(t *testing.T) {
	run := func(id catalog.ID, exists func(string) bool) (install.ItemResult, [][]string) {
		t.Helper()
		plans, err := install.ResolveAll([]catalog.ID{id}, "dnf")
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildSteps(plans, 0, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 0, nil }}
		r := &install.Runner{Exec: ex, EUID: 0, Force: false, Exists: exists}
		results, code := r.RunSteps(context.Background(), steps)
		if code != 0 || len(results) != 1 {
			t.Fatalf("%s code %d %+v", id, code, results)
		}
		return results[0], ex.calls
	}
	res, calls := run("compiler", func(p string) bool { return p == "/usr/bin/gcc" })
	if res.Status != "ok" || len(calls) != 1 || !slices.Equal(calls[0], []string{"dnf", "install", "-y", "gcc", "gcc-c++"}) {
		t.Fatalf("gcc only %+v %v", res, calls)
	}
	res, calls = run("compiler", func(p string) bool { return p == "/usr/bin/gcc" || p == "/usr/bin/g++" })
	if res.Status != "satisfied" || res.Argv != nil || len(calls) != 0 {
		t.Fatalf("both compilers %+v %v", res, calls)
	}
	res, calls = run("rust", func(p string) bool { return p == "/usr/bin/rustc" })
	if res.Status != "ok" || len(calls) != 1 || !slices.Equal(calls[0], []string{"dnf", "install", "-y", "rust", "cargo"}) {
		t.Fatalf("rustc only %+v %v", res, calls)
	}
	res, calls = run("go", func(p string) bool { return p == "/usr/local/go" || p == "/usr/local/go/bin/go" })
	if res.Status != "ok" || len(calls) != 1 || !slices.Equal(calls[0], []string{"dnf", "install", "-y", "golang"}) {
		t.Fatalf("tarball path %+v %v", res, calls)
	}
}

func TestForceDoesNotSkipReleaseOrGo(t *testing.T) {
	home := t.TempDir()
	releaseCalled := false
	steps := []install.Step{
		{ItemID: "mitos", Title: "release mitos", Kind: install.KindRelease, Method: pm.MethodRelease, ReleaseRepo: "mitos-editor/mitos"},
		{ItemID: "go", Title: "go-tarball go", Kind: install.KindGo, Method: pm.MethodGoTarball},
	}
	r := &install.Runner{
		Force:  true,
		Home:   home,
		Exists: func(string) bool { return true },
		Release: func(context.Context, string, string, release.AssetQuery) error {
			releaseCalled = true
			return nil
		},
		GoInstall: func(context.Context, release.AssetQuery, int, string) error {
			return fmt.Errorf("/usr/local/go 已存在，不覆蓋")
		},
	}
	results, code := r.RunSteps(context.Background(), steps)
	if !releaseCalled || code != 1 || len(results) != 2 {
		t.Fatalf("code %d release %v %+v", code, releaseCalled, results)
	}
	if results[0].Status != "ok" || results[1].Status != "failed" || !strings.Contains(results[1].ErrText, "已存在，不覆蓋") {
		t.Fatalf("%+v", results)
	}
}

// With no config source (Repo empty) the config step fails with the reason
// instead of reporting ok after copying nothing; the failure reaches the
// progress line and the install log.
func TestConfigStepFailsWithoutRepo(t *testing.T) {
	plans, err := install.ResolveAll([]catalog.ID{"starship"}, "dnf")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ repoErr, want string }{
		{"", "設定檔來源不存在"},
		{"設定檔解壓失敗: disk full", "設定檔解壓失敗: disk full"},
	} {
		home := t.TempDir()
		steps, err := install.BuildSteps(plans, 0, home)
		if err != nil {
			t.Fatal(err)
		}
		var progress []string
		var logged []install.ItemResult
		r := &install.Runner{
			Home:     home,
			RepoErr:  tc.repoErr,
			Exists:   func(string) bool { return false },
			Release:  func(context.Context, string, string, release.AssetQuery) error { return nil },
			Deploy:   configcopy.Deploy,
			Rc:       shellrc.EnsureFile,
			Progress: func(title, status string) { progress = append(progress, title+"="+status) },
			Log:      func(res []install.ItemResult) error { logged = res; return nil },
		}
		results, code := r.RunSteps(context.Background(), steps)
		if code == 0 {
			t.Fatalf("%q: code 0 %+v", tc.repoErr, results)
		}
		var got install.ItemResult
		for _, res := range logged {
			if res.ID == "starship" {
				got = res
			}
		}
		if got.Status != "failed" || got.ErrText != tc.want {
			t.Fatalf("%q: logged %+v", tc.repoErr, got)
		}
		failed := false
		for _, p := range progress {
			if strings.HasSuffix(p, "=failed") {
				failed = true
			}
		}
		if !failed {
			t.Fatalf("%q: progress %v", tc.repoErr, progress)
		}
	}
}
