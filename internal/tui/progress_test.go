package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

type fakeSudo struct {
	mu       sync.Mutex
	cached   bool
	wantPW   string
	noCache  bool // Validate succeeds but sudo keeps no cache
	got      []string
	refreshN int
}

func (f *fakeSudo) Cached(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cached
}

func (f *fakeSudo) Validate(_ context.Context, pw []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, string(pw))
	if string(pw) != f.wantPW {
		return false, errors.New("sudo 驗證失敗：Sorry, try again.")
	}
	if f.noCache {
		return false, nil
	}
	f.cached = true
	return true, nil
}

func (f *fakeSudo) Refresh(context.Context) {
	f.mu.Lock()
	f.refreshN++
	f.mu.Unlock()
}

func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestSplitLines(t *testing.T) {
	type ln struct {
		s  string
		cr bool
	}
	var got []ln
	splitLines(strings.NewReader("a\r\nb\n\x1b[31mred\x1b[0m\n10%\r50%\r100%\ntab\there\nlast"), func(s string, cr bool) {
		got = append(got, ln{s, cr})
	})
	want := []ln{{"a", false}, {"b", false}, {"red", false}, {"10%", true}, {"50%", true}, {"100%", false}, {"tab    here", false}, {"last", false}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d: %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestSudoArgvNeverPromptsOnTheTerminal(t *testing.T) {
	if got := strings.Join(sudoArgv([]string{"sudo", "dnf", "install", "-y", "jq"}, sudoCached), " "); got != "sudo -n dnf install -y jq" {
		t.Fatal(got)
	}
	if got := sudoArgv([]string{"sudo", "rm", "-rf", "/usr/local/go"}, sudoStdin); strings.Join(got, "|") != "sudo|-S|-p||rm|-rf|/usr/local/go" {
		t.Fatalf("%q", got)
	}
	if got := strings.Join(sudoArgv([]string{"brew", "install", "jq"}, sudoStdin), " "); got != "brew install jq" {
		t.Fatal(got)
	}
}

func collect(ch chan bridgeMsg) func() []bridgeMsg {
	var mu sync.Mutex
	var got []bridgeMsg
	done := make(chan struct{})
	go func() {
		for msg := range ch {
			mu.Lock()
			got = append(got, msg)
			mu.Unlock()
		}
		close(done)
	}()
	return func() []bridgeMsg {
		close(ch)
		<-done
		mu.Lock()
		defer mu.Unlock()
		return got
	}
}

func TestStreamExecStreamsStdoutAndStderr(t *testing.T) {
	ch := make(chan bridgeMsg)
	stop := collect(ch)
	out, code, err := streamExec{ch: ch}.Run(context.Background(), []string{"sh", "-c", "echo one; echo two >&2; printf 'p1\\rp2\\n'; exit 3"})
	msgs := stop()
	if err != nil || code != 3 {
		t.Fatalf("code %d err %v", code, err)
	}
	if !strings.Contains(out, "one") || !strings.Contains(out, "two") {
		t.Fatalf("combined %q", out)
	}
	var lines []string
	for _, m := range msgs {
		if m.kind != kindOutput {
			t.Fatalf("kind %d", m.kind)
		}
		lines = append(lines, m.line)
	}
	if strings.Join(lines, ",") != "one,two,p1,p2" || !msgs[2].cr {
		t.Fatalf("lines %v", lines)
	}
}

// TestStreamExecSudoModes uses a stand-in "sudo" on PATH that reports its
// arguments and the length of what it read on stdin (never the text).
func TestStreamExecSudoModes(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"args:$*\"\nif read -r pw; then echo \"len:${#pw}\"; else echo nostdin; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ch := make(chan bridgeMsg)
	stop := collect(ch)
	out, code, err := streamExec{ch: ch, mode: sudoCached}.Run(context.Background(), []string{"sudo", "dnf", "remove", "-y", "jq"})
	stop()
	if err != nil || code != 0 || !strings.Contains(out, "args:-n dnf remove -y jq") || !strings.Contains(out, "nostdin") {
		t.Fatalf("cached %q %d %v", out, code, err)
	}

	ch = make(chan bridgeMsg)
	stop = collect(ch)
	out, code, err = streamExec{ch: ch, mode: sudoStdin, pw: []byte("hunter22")}.Run(context.Background(), []string{"sudo", "dnf", "remove", "-y", "jq"})
	msgs := stop()
	if err != nil || code != 0 || !strings.Contains(out, "args:-S -p  dnf remove -y jq") || !strings.Contains(out, "len:8") {
		t.Fatalf("stdin %q %d %v", out, code, err)
	}
	for _, m := range msgs {
		if strings.Contains(m.line, "hunter22") {
			t.Fatal("password reached the UI")
		}
	}
}

func TestStreamExecCancel(t *testing.T) {
	ch := make(chan bridgeMsg, 16)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for msg := range ch {
			if msg.line == "ready" {
				cancel()
			}
		}
	}()
	_, _, err := streamExec{ch: ch}.Run(ctx, []string{"sh", "-c", "echo ready; exec sleep 30"})
	close(ch)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}

func progressFixture() model {
	m := New(fedoraInfo())
	m.width, m.height = 80, 30
	m.phase = phaseInstalling
	m.runID = 3
	m.events = make(chan bridgeMsg, 1)
	m.steps = []install.Step{{Title: "update dnf"}, {Title: "install jq"}, {Title: "release starship"}, {Title: "config starship"}}
	m.stepState = make([]string, len(m.steps))
	m.deps.Detect = func(string) map[catalog.ID]bool { return map[catalog.ID]bool{"jq": true} }
	return m
}

func feed(t *testing.T, m model, msgs ...bridgeMsg) model {
	t.Helper()
	for _, msg := range msgs {
		msg.runID = m.runID
		next, cmd := m.Update(msg)
		m = next.(model)
		if msg.kind != kindDone && cmd == nil {
			t.Fatal("stopped listening mid-run")
		}
	}
	return m
}

func TestProgressFollowsStreamedMessages(t *testing.T) {
	m := progressFixture()
	m = feed(t, m,
		bridgeMsg{kind: kindProgress, title: "update dnf"},
		bridgeMsg{kind: kindOutput, line: "Updating metadata"},
		bridgeMsg{kind: kindProgress, title: "update dnf", status: "ok"},
		bridgeMsg{kind: kindProgress, title: "install jq"},
		bridgeMsg{kind: kindOutput, line: "10%", cr: true},
		bridgeMsg{kind: kindOutput, line: "100%"},
		bridgeMsg{kind: kindOutput, line: "Installed jq"},
	)
	if got := strings.Join(m.stepState, ","); got != "ok,running,," {
		t.Fatalf("states %q", got)
	}
	if cur, total := m.stepCounter(); cur != 2 || total != 4 {
		t.Fatalf("counter %d/%d", cur, total)
	}
	if strings.Join(m.output, "|") != "Updating metadata|100%|Installed jq" {
		t.Fatalf("output %q", m.output)
	}
	view := render(m, false)
	for _, s := range []string{"安裝中  2/4  install jq", "完成    update dnf", "執行中  install jq", "等待    release starship", "輸出", "  Installed jq", "操作  ctrl+c 取消"} {
		if !strings.Contains(view, s) {
			t.Fatalf("missing %q\n%s", s, view)
		}
	}
	if strings.Contains(view, "10%") {
		t.Fatal("a \\r line was not replaced")
	}
	// A fallback announces itself with a new title; it belongs to the running step.
	m = feed(t, m,
		bridgeMsg{kind: kindProgress, title: "install jq"},
		bridgeMsg{kind: kindProgress, title: "install jq", status: "ok"},
		bridgeMsg{kind: kindProgress, title: "release starship"},
		bridgeMsg{kind: kindProgress, title: "release-fallback starship"},
		bridgeMsg{kind: kindProgress, title: "release-fallback starship", status: "failed"},
		bridgeMsg{kind: kindProgress, title: "config starship", status: "skipped"},
	)
	if got := strings.Join(m.stepState, ","); got != "ok,ok,failed,skipped" {
		t.Fatalf("states %q", got)
	}
	m = feed(t, m, bridgeMsg{kind: kindDone, code: 1, results: []install.ItemResult{
		{ID: "jq", Status: "ok"}, {ID: "starship", Status: "failed", ErrText: "404"},
	}})
	if m.phase != phaseReport || m.exitCode != 1 {
		t.Fatalf("phase %d", m.phase)
	}
	rep := render(m, false)
	for _, s := range []string{"安裝完成：成功 1、失敗 1", "starship  failed", "操作  任意鍵回主頁", "  Installed jq"} {
		if !strings.Contains(rep, s) {
			t.Fatalf("missing %q\n%s", s, rep)
		}
	}
}

func TestProgressStaysInWindow(t *testing.T) {
	m := progressFixture()
	for i := 0; i < 1000; i++ {
		m.appendOutput(strings.Repeat("長", 100), false)
	}
	if len(m.output) != outputCap {
		t.Fatalf("output kept %d", len(m.output))
	}
	for _, size := range [][2]int{{80, 30}, {40, 10}, {20, 5}, {12, 4}} {
		m.width, m.height = size[0], size[1]
		for _, styled := range []bool{false, true} {
			var view string
			if styled {
				view = renderStyled(m, testRenderer())
			} else {
				view = render(m, false)
			}
			lines := frameLines(view)
			if len(lines) != size[1] {
				t.Fatalf("%v styled=%v: %d lines", size, styled, len(lines))
			}
			for _, ln := range lines {
				// The plain footer is not cut (unchanged behaviour), so only the body is checked there.
				if w := ansi.StringWidth(ln); w > size[0] && (styled || ln != lines[len(lines)-1]) {
					t.Fatalf("%v styled=%v: line width %d %q", size, styled, w, ln)
				}
			}
		}
	}
}

func TestCtrlCCancelsTheRun(t *testing.T) {
	m := progressFixture()
	m.steps = []install.Step{{ItemID: "jq", Title: "install jq", Kind: install.KindPackage, Method: "package", Manager: "dnf", Argv: []string{"sudo", "dnf", "install", "-y", "jq"}}}
	started := make(chan struct{})
	m.deps.Runner.Exec = blockExec{started: started}
	m.deps.Sudo = &fakeSudo{cached: true}
	next, cmd := m.launch(sudoCached, nil)
	m = next.(model)
	msg := cmd().(bridgeMsg) // progress start
	next, cmd = m.Update(msg)
	m = next.(model)
	<-started
	next, c := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(model)
	if c != nil || !m.canceled || m.phase != phaseInstalling {
		t.Fatal("ctrl+c")
	}
	if !strings.Contains(render(m, false), "取消中") {
		t.Fatal(render(m, false))
	}
	m, _ = drain(t, m, cmd)
	if m.phase != phaseReport || !m.canceled {
		t.Fatalf("phase %d", m.phase)
	}
	if !strings.Contains(render(m, false), "安裝已取消") {
		t.Fatal(render(m, false))
	}
	if m.stepState[0] != stCanceled {
		t.Fatalf("state %q", m.stepState[0])
	}
}

type blockExec struct{ started chan struct{} }

func (b blockExec) Run(ctx context.Context, _ []string) (string, int, error) {
	close(b.started)
	<-ctx.Done()
	return "", 0, context.Canceled
}

func TestAnyKeyOnReportReturnsHomeAndRedetects(t *testing.T) {
	keys := []tea.KeyMsg{
		keyRune('x'), keyRune('q'), keyRune('y'), keyRune('n'),
		{Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyCtrlC}, {Type: tea.KeySpace}, {Type: tea.KeyDown},
	}
	for _, k := range keys {
		m := progressFixture()
		m.finishRun(nil, 0)
		next, cmd := m.Update(k)
		mm := next.(model)
		if mm.phase != phaseHome || cmd == nil {
			t.Fatalf("%s: phase %d", k.String(), mm.phase)
		}
		next, _ = mm.Update(cmd())
		if !next.(model).installed["jq"] {
			t.Fatalf("%s: home did not re-detect", k.String())
		}
	}
}

func TestInstallEnterStartsWithoutConfirm(t *testing.T) {
	m := New(fedoraInfo())
	m.width, m.height = 80, 30
	m.installed = map[catalog.ID]bool{}
	for id := range m.selected {
		m.selected[id] = false
	}
	m.selected["jq"] = true
	rec := &recordExec{}
	m.deps.Runner.Exec = rec
	m.deps.Runner.Home = t.TempDir()
	m.deps.Runner.Exists = func(string) bool { return false }
	m.deps.Sudo = &fakeSudo{cached: true}
	m.deps.Detect = func(string) map[catalog.ID]bool { return nil }
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.phase == phaseSudo {
		next, cmd = m.Update(cmd())
		m = next.(model)
	}
	if m.phase != phaseInstalling || cmd == nil {
		t.Fatalf("phase %d", m.phase)
	}
	m, _ = drain(t, m, cmd)
	calls := rec.calls()
	if len(calls) == 0 || !strings.Contains(strings.Join(calls[len(calls)-1], " "), "dnf install -y jq") {
		t.Fatalf("calls %v", calls)
	}
	if os.Geteuid() != 0 && calls[len(calls)-1][0] != "sudo" {
		t.Fatalf("sudo dropped %v", calls)
	}
	if m.phase != phaseReport {
		t.Fatalf("phase %d", m.phase)
	}
}

func TestSudoPasswordPage(t *testing.T) {
	m := progressFixture()
	m.phase = phaseBrowsing
	fs := &fakeSudo{wantPW: "pässwörd"}
	m.deps.Sudo = fs
	m.deps.Runner.Exec = &recordExec{}
	next, cmd := m.needSudo(true)
	m = next.(model)
	if m.phase != phaseSudo || !m.sudoBusy || cmd == nil {
		t.Fatal("check")
	}
	if got := typeText(t, m, "abc"); len(got.pw) != 0 {
		t.Fatal("typed while checking")
	}
	next, _ = m.Update(cmd())
	m = next.(model)
	if m.phase != phaseSudo || m.sudoBusy {
		t.Fatal("password page")
	}
	m = typeText(t, m, "wrongx")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	view := render(m, false)
	if !strings.Contains(view, "sudo 密碼：*****") || strings.Contains(view, "wrong") || !strings.Contains(view, "操作  enter 確認  esc 返回") {
		t.Fatal(view)
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.pw != nil || !m.sudoBusy || cmd == nil {
		t.Fatal("submit must clear the field")
	}
	next, _ = m.Update(cmd())
	m = next.(model)
	if m.phase != phaseSudo || !strings.Contains(render(m, false), "Sorry, try again.") {
		t.Fatal(render(m, false))
	}
	m = typeText(t, m, "pässwörd")
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	next, cmd = m.Update(cmd())
	m = next.(model)
	if m.phase != phaseInstalling || cmd == nil {
		t.Fatalf("phase %d", m.phase)
	}
	if strings.Join(fs.got, ",") != "wrong,pässwörd" {
		t.Fatalf("validated %q", fs.got)
	}
	drain(t, m, cmd)

	// esc leaves the page and wipes what was typed.
	m = progressFixture()
	m.phase = phaseUninstall
	m.mode = modeUninstall
	m.deps.Sudo = &fakeSudo{}
	next, cmd = m.needSudo(true)
	m = next.(model)
	next, _ = m.Update(cmd())
	m = typeText(t, next.(model), "abc")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseUninstall || m.pw != nil {
		t.Fatalf("esc phase %d", m.phase)
	}
}

func TestSudoWithoutCacheFeedsStdin(t *testing.T) {
	m := progressFixture()
	m.phase = phaseBrowsing
	m.deps.Sudo = &fakeSudo{wantPW: "pw", noCache: true}
	m.deps.Runner.Exec = &recordExec{}
	next, cmd := m.needSudo(true)
	next, _ = next.(model).Update(cmd())
	m = typeText(t, next.(model), "pw")
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(sudoValidMsg)
	if msg.err != nil || msg.cached || string(msg.pw) != "pw" {
		t.Fatalf("%+v", msg.err)
	}
	next, cmd = next.(model).Update(msg)
	m = next.(model)
	if m.phase != phaseInstalling || m.pw != nil {
		t.Fatal("run")
	}
	drain(t, m, cmd)
	if string(msg.pw) != "\x00\x00" {
		t.Fatalf("password not zeroed after the run: %q", msg.pw)
	}
}

// TestNoYStepsRemain: no page asks for y any more, and no code path reads it.
func TestNoYStepsRemain(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{`case "y"`, `"y", "n"`, "按 y", "y 確定", "y 開始", "要強制重新下載嗎", "只列出已安裝的工具", "不可移除"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s still has %q", f, bad)
			}
		}
	}
	base := progressFixture()
	base.installed = map[catalog.ID]bool{"python": true, "jq": true}
	for _, ph := range []phase{phaseHome, phaseBrowsing, phaseUninstall, phaseSudo, phaseInstalling, phaseReport} {
		m := base
		m.phase = ph
		m.warnings = []string{"警告"}
		view := render(m, false)
		if strings.Contains(view, " y ") || strings.Contains(view, "y/n") || strings.Contains(view, "n/esc") {
			t.Errorf("phase %d shows a y prompt\n%s", ph, view)
		}
	}
}
