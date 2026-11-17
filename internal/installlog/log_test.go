package installlog_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/installlog"
)

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	rec := installlog.Record{
		Time:   "2026-11-10T00:00:00Z",
		GOOS:   "linux",
		GOARCH: "amd64",
		PM:     "dnf",
		DryRun: true,
		Items: []installlog.Item{{
			ID:     "wget",
			Method: "package",
			Argv:   []string{"sudo", "dnf", "install", "-y", "wget2-wget"},
			Status: "ok",
		}},
	}
	if err := installlog.Write(dir, rec, false, nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "last-install.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got installlog.Record
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != 1 || got.DryRun || got.PM != "dnf" || got.Time != rec.Time {
		t.Fatalf("%+v", got)
	}
	if len(got.Items) != 1 || strings.Join(got.Items[0].Argv, " ") != "sudo dnf install -y wget2-wget" {
		t.Fatalf("%+v", got.Items)
	}
	logBody, err := os.ReadFile(filepath.Join(dir, "install.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBody), "2026-11-10T00:00:00Z wget package ok") {
		t.Fatalf("log %s", logBody)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("dir mode %o", info.Mode().Perm())
	}
}

func TestWriteOmitsBashrc(t *testing.T) {
	dir := t.TempDir()
	rec := installlog.Record{
		Schema: 1,
		Time:   "2026-11-10T00:00:00Z",
		GOOS:   "linux",
		GOARCH: "amd64",
		PM:     "dnf",
		Items:  []installlog.Item{{ID: "git", Method: "package", Status: "ok"}},
	}
	if err := installlog.Write(dir, rec, false, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"install.log", "last-install.json"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `eval "$(starship`) || strings.Contains(string(b), "# dotfiles:") {
			t.Fatalf("%s contains bashrc text", name)
		}
	}
	bad := rec
	bad.Items = []installlog.Item{{ID: "git", Method: "package", Status: "failed", Err: "# dotfiles:starship"}}
	if err := installlog.Write(dir, bad, false, nil); err == nil {
		t.Fatal("expected reject")
	}
}

func TestVerboseArgv(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	rec := installlog.Record{
		Schema: 1,
		Time:   "2026-11-10T00:00:00Z",
		GOOS:   "linux",
		GOARCH: "amd64",
		PM:     "dnf",
		Items: []installlog.Item{{
			ID:     "wget",
			Method: "package",
			Argv:   []string{"sudo", "dnf", "install", "-y", "wget2-wget"},
			Status: "ok",
		}},
	}
	if err := installlog.Write(dir, rec, true, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "sudo dnf install -y wget2-wget") {
		t.Fatalf("stderr %q", stderr.String())
	}
}
