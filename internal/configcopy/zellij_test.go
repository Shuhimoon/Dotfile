package configcopy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/configcopy"
)

// config.kdl loads zjstatus from wherever zellij/ was deployed: ~/.config
// without XDG_CONFIG_HOME, $XDG_CONFIG_HOME when it is absolute. A redeploy
// is unchanged (no new backup) and Undeploy still sees it as the repo's file.
func TestZellijConfigPointsAtDeployedPlugin(t *testing.T) {
	const kdl = "plugins {\n    compact-bar location=\"file:~/.config/zellij/plugins/zjstatus.wasm\" {\n    }\n}\n"
	for _, xdgSet := range []bool{false, true} {
		repo := fakeRepo(t)
		writeFile(t, filepath.Join(repo, "zellij", "config.kdl"), kdl)
		home := t.TempDir()
		xdg, cfg := "", filepath.Join(home, ".config")
		roots := []string{home}
		if xdgSet {
			xdg = t.TempDir()
			cfg = xdg
			roots = append(roots, xdg)
		}
		files, err := configcopy.FilesFor("zellij", repo, home, xdg)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 18, 1, 2, 3, 0, time.UTC)
		var conf configcopy.File
		for _, f := range files {
			if filepath.Base(f.Src) == "config.kdl" {
				conf = f
				if f.Policy != configcopy.PolicyZellijConfig {
					t.Fatalf("policy %v", f.Policy)
				}
			} else if f.Policy != configcopy.PolicyBackupReplace {
				t.Fatalf("%s policy %v", f.Src, f.Policy)
			}
			if _, err := configcopy.Deploy(f.Src, f.Dst, f.Policy, now, roots...); err != nil {
				t.Fatal(err)
			}
		}
		want := "file:" + filepath.Join(cfg, "zellij", "plugins", "zjstatus.wasm")
		got, err := os.ReadFile(filepath.Join(cfg, "zellij", "config.kdl"))
		if err != nil || !strings.Contains(string(got), `location="`+want+`"`) || strings.Contains(string(got), "~") {
			t.Fatalf("xdg=%v: %q %v", xdgSet, got, err)
		}
		if _, err := os.Stat(strings.TrimPrefix(want, "file:")); err != nil {
			t.Fatalf("xdg=%v: plugin not at the path config.kdl uses: %v", xdgSet, err)
		}
		if src, _ := os.ReadFile(conf.Src); string(src) != kdl {
			t.Fatal("repo config.kdl changed")
		}
		res, err := configcopy.Deploy(conf.Src, conf.Dst, conf.Policy, now.Add(time.Hour), roots...)
		if err != nil || res.Action != "unchanged" {
			t.Fatalf("xdg=%v: redeploy %v %v", xdgSet, res, err)
		}
		if baks, _ := filepath.Glob(conf.Dst + ".bak.*"); len(baks) != 0 {
			t.Fatalf("xdg=%v: backups %v", xdgSet, baks)
		}
		res, err = configcopy.Undeploy(conf, now, roots...)
		if err != nil || res.Action != "removed" {
			t.Fatalf("xdg=%v: undeploy %v %v", xdgSet, res, err)
		}
	}
}
