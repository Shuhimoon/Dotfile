package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempIO(t *testing.T) (in, stdout, stderr *os.File) {
	t.Helper()
	dir := t.TempDir()
	var err error
	in, err = os.Create(filepath.Join(dir, "in"))
	if err != nil {
		t.Fatal(err)
	}
	stdout, err = os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	stderr, err = os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		in.Close()
		stdout.Close()
		stderr.Close()
	})
	return in, stdout, stderr
}

func readFromStart(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestUsageMentionsForce(t *testing.T) {
	in, stdout, stderr := tempIO(t)
	if code := run([]string{"-h"}, in, stdout, stderr); code != 0 {
		t.Fatalf("code %d", code)
	}
	got := readFromStart(t, stdout)
	if got != usage || !strings.Contains(got, "tools [--verbose] [--force]") || !strings.Contains(got, "[--verbose] [--force]") {
		t.Fatalf("%q", got)
	}
	in, stdout, stderr = tempIO(t)
	if code := run([]string{"--force", "--verbose", "apply", "--help"}, in, stdout, stderr); code != 0 || readFromStart(t, stdout) != usage {
		t.Fatalf("apply help %q", readFromStart(t, stdout))
	}
}

func TestFrontForceFlags(t *testing.T) {
	in, stdout, stderr := tempIO(t)
	if code := run([]string{"--force", "--verbose"}, in, stdout, stderr); code != 2 || readFromStart(t, stderr) != usage {
		t.Fatalf("non-tty %d %q", code, readFromStart(t, stderr))
	}
	in, stdout, stderr = tempIO(t)
	if code := run([]string{"--verbose", "--force"}, in, stdout, stderr); code != 2 || readFromStart(t, stderr) != usage {
		t.Fatalf("order %d %q", code, readFromStart(t, stderr))
	}
	in, stdout, stderr = tempIO(t)
	if code := run([]string{"--force", "--force"}, in, stdout, stderr); code != 2 || !strings.Contains(readFromStart(t, stderr), "未知參數 --force") {
		t.Fatalf("dup %q", readFromStart(t, stderr))
	}
	in, stdout, stderr = tempIO(t)
	if code := run([]string{"--force=true"}, in, stdout, stderr); code != 2 || !strings.Contains(readFromStart(t, stderr), "未知參數 --force=true") {
		t.Fatalf("eq %q", readFromStart(t, stderr))
	}
	in, stdout, stderr = tempIO(t)
	if code := run([]string{"apply", "--force=true"}, in, stdout, stderr); code != 2 || !strings.Contains(readFromStart(t, stderr), "未知參數 --force=true") {
		t.Fatalf("apply eq %q", readFromStart(t, stderr))
	}
}

func TestApplyForceMatchesDryRun(t *testing.T) {
	runOnce := func(args []string) (int, string, string) {
		t.Helper()
		in, stdout, stderr := tempIO(t)
		code := run(args, in, stdout, stderr)
		return code, readFromStart(t, stdout), readFromStart(t, stderr)
	}
	base := []string{"apply", "--dry-run", "--only", "whois", "--pm", "brew"}
	c1, out1, err1 := runOnce(base)
	c2, out2, err2 := runOnce(append(append([]string{}, base...), "--force"))
	c3, out3, err3 := runOnce(append([]string{"--force"}, base...))
	if c1 != c2 || c1 != c3 || out1 != out2 || out1 != out3 || err1 != err2 || err1 != err3 {
		t.Fatalf("base %d %q %q\nflag %d %q %q\nfront %d %q %q", c1, out1, err1, c2, out2, err2, c3, out3, err3)
	}
	if strings.Contains(err2, "未知參數") || strings.Contains(err3, "未知參數") {
		t.Fatal(err2)
	}
}

func TestApplyConfigOnlyDryRun(t *testing.T) {
	runOnce := func(args ...string) (int, string, string) {
		t.Helper()
		in, stdout, stderr := tempIO(t)
		code := run(args, in, stdout, stderr)
		return code, readFromStart(t, stdout), readFromStart(t, stderr)
	}
	code, out, _ := runOnce("apply", "--config-only", "--dry-run")
	if code != 0 || out != "config zsh\nconfig starship\nconfig zellij\nzsh-plugins\n" {
		t.Fatalf("all: %d %q", code, out)
	}
	code, out, _ = runOnce("apply", "--config-only", "--only", "zellij,starship", "--dry-run")
	if code != 0 || out != "config starship\nconfig zellij\n" {
		t.Fatalf("only: %d %q", code, out)
	}
	code, out, _ = runOnce("apply", "--dry-run", "--only=zsh", "--config-only")
	if code != 0 || out != "config zsh\nzsh-plugins\n" {
		t.Fatalf("zsh: %d %q", code, out)
	}
	for _, only := range []string{"git", "zellij,nope", ""} {
		code, out, errOut := runOnce("apply", "--config-only", "--only", only, "--dry-run")
		if code != 2 || out != "" || errOut == "" {
			t.Fatalf("--only %q: %d %q %q", only, code, out, errOut)
		}
	}
}

// A real --config-only run from this checkout deploys into $HOME without a
// package manager or sudo, and reports each tool.
func TestApplyConfigOnlyDeploys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("SUDO_USER", "")
	t.Setenv("PATH", "")
	in, stdout, stderr := tempIO(t)
	if code := run([]string{"apply", "--config-only", "--only", "starship,zellij"}, in, stdout, stderr); code != 0 {
		t.Fatalf("code %d %q %q", code, readFromStart(t, stdout), readFromStart(t, stderr))
	}
	if got := readFromStart(t, stdout); got != "starship  ok  只部署設定\nzellij  ok  只部署設定\n" {
		t.Fatalf("stdout %q", got)
	}
	kdl, err := os.ReadFile(filepath.Join(home, ".config", "zellij", "config.kdl"))
	if err != nil || !strings.Contains(string(kdl), "file:"+filepath.Join(home, ".config", "zellij", "plugins", "zjstatus.wasm")) {
		t.Fatalf("config.kdl %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "starship.toml")); err != nil {
		t.Fatal(err)
	}
}
