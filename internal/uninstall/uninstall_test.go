package uninstall_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/shellrc"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

type fakeExec struct {
	calls [][]string
	fn    func([]string) (string, int, error)
}

func (f *fakeExec) Run(_ context.Context, argv []string) (string, int, error) {
	f.calls = append(f.calls, append([]string(nil), argv...))
	if f.fn != nil {
		return f.fn(argv)
	}
	return "", 0, nil
}

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func set(paths ...string) func(string) bool {
	m := map[string]bool{}
	for _, p := range paths {
		m[p] = true
	}
	return func(p string) bool { return m[p] }
}

func TestRemovable(t *testing.T) {
	locked := []catalog.ID{"git", "curl", "wget", "unzip", "rsync", "vim", "less", "net-tools", "whois", "dnsutils", "traceroute"}
	for _, id := range locked {
		if uninstall.Removable(id) {
			t.Errorf("%s removable", id)
		}
	}
	for _, id := range []catalog.ID{"zsh", "starship", "eza", "fzf", "zellij", "mitos", "go", "rust", "python", "node", "make", "pkg-config", "docker", "podman", "zig"} {
		if !uninstall.Removable(id) {
			t.Errorf("%s not removable", id)
		}
	}
	if uninstall.Removable("nope") {
		t.Error("unknown id removable")
	}
	if _, ok := uninstall.NeedsWarning("python"); !ok {
		t.Error("python needs a warning")
	}
	if _, ok := uninstall.NeedsWarning("node"); ok {
		t.Error("node warning")
	}
	for _, mgr := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "xbps", "brew"} {
		if !uninstall.Supported("docker", mgr) || !uninstall.Supported("podman", mgr) {
			t.Errorf("containers unsupported on %s", mgr)
		}
	}
	if uninstall.Supported("compiler", "brew") || uninstall.Supported("make", "brew") || !uninstall.Supported("compiler", "dnf") {
		t.Error("Supported")
	}
}

func TestBuildStepsRejectsBasics(t *testing.T) {
	if _, err := uninstall.BuildSteps([]catalog.ID{"jq", "git"}, "dnf", 1000, uninstall.Probe{Home: "/h"}); err == nil {
		t.Fatal("git accepted")
	}
	if _, err := uninstall.BuildSteps([]catalog.ID{"compiler"}, "brew", 501, uninstall.Probe{Home: "/h"}); err == nil {
		t.Fatal("xcode compiler accepted")
	}
}

func TestBuildSteps(t *testing.T) {
	h := "/home/u"
	type want struct{ title, argv string }
	cases := []struct {
		name  string
		ids   []catalog.ID
		mgr   string
		euid  int
		probe uninstall.Probe
		want  []want
	}{
		{"dnf package + config + rc", []catalog.ID{"fzf"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/fzf")},
			[]want{{"remove fzf", "sudo dnf remove -y fzf"}, {"shell-rc fzf", ""}}},
		{"python warns but builds", []catalog.ID{"python"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/python3")},
			[]want{{"remove python", "sudo dnf remove -y python3"}}},
		{"release starship on dnf", []catalog.ID{"starship"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set(h + "/.local/bin/starship")},
			[]want{{"release-remove starship", ""}, {"shell-rc starship", ""}, {"config starship", ""}}},
		{"package and release both present", []catalog.ID{"ripgrep"}, "apt", 0,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/rg", h+"/.local/bin/rg")},
			[]want{{"remove ripgrep", "apt-get remove -y ripgrep"}, {"release-remove ripgrep", ""}}},
		{"nothing on disk falls back to package manager", []catalog.ID{"jq"}, "pacman", 1000,
			uninstall.Probe{Home: h},
			[]want{{"remove jq", "sudo pacman -R --noconfirm jq"}}},
		{"docker on fedora removes moby", []catalog.ID{"docker"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/docker")},
			[]want{{"remove docker", "sudo dnf remove -y moby-engine docker-compose"}}},
		{"docker on ubuntu removes docker.io", []catalog.ID{"docker"}, "apt", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/docker")},
			[]want{{"remove docker", "sudo apt-get remove -y docker.io"}}},
		{"podman on arch", []catalog.ID{"podman"}, "pacman", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/podman")},
			[]want{{"remove podman", "sudo pacman -R --noconfirm podman"}}},
		{"zig tarball on ubuntu removes the tree", []catalog.ID{"zig"}, "apt", 1000,
			uninstall.Probe{Home: h, Exists: set(h + "/.local/bin/zig")},
			[]want{{"release-remove zig", ""}}},
		{"zig package on fedora", []catalog.ID{"zig"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/zig")},
			[]want{{"remove zig", "sudo dnf remove -y zig"}}},
		{"zig on fedora from the tarball fallback", []catalog.ID{"zig"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set(h + "/.local/bin/zig")},
			[]want{{"release-remove zig", ""}}},
		{"go tarball non-root uses sudo rm", []catalog.ID{"go"}, "dnf", 1000,
			uninstall.Probe{Home: h, GoRoot: "/usr/local", Exists: set("/usr/local/go/bin/go")},
			[]want{{"go-tarball-remove go", "sudo rm -rf /usr/local/go"}, {"shell-rc go", ""}}},
		{"go tarball as root removes in-process", []catalog.ID{"go"}, "dnf", 0,
			uninstall.Probe{Home: h, Exists: set("/usr/local/go/bin/go")},
			[]want{{"go-tarball-remove go", ""}, {"shell-rc go", ""}}},
		{"rustup", []catalog.ID{"rust"}, "dnf", 1000,
			uninstall.Probe{Home: h, Exists: set(h + "/.cargo/bin/rustup")},
			[]want{{"rustup-remove rust", h + "/.cargo/bin/rustup self uninstall -y"}, {"shell-rc rust", ""}}},
		{"brew no sudo, catalog order", []catalog.ID{"node", "zellij"}, "brew", 501,
			uninstall.Probe{Home: h, Exists: set("/opt/homebrew/bin/zellij", "/opt/homebrew/bin/node")},
			[]want{{"remove zellij", "brew uninstall zellij"}, {"config zellij", ""}, {"remove node", "brew uninstall node"}}},
		{"zsh", []catalog.ID{"zsh"}, "zypper", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/zsh")},
			[]want{{"remove zsh", "sudo zypper --non-interactive remove zsh"}, {"config zsh", ""}}},
		{"apk + xbps", []catalog.ID{"bat"}, "xbps", 1000,
			uninstall.Probe{Home: h, Exists: set("/usr/bin/bat")},
			[]want{{"remove bat", "sudo xbps-remove -y bat"}}},
	}
	for _, c := range cases {
		steps, err := uninstall.BuildSteps(c.ids, c.mgr, c.euid, c.probe)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []want
		for _, st := range steps {
			got = append(got, want{st.Title, strings.Join(st.Argv, " ")})
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: %v", c.name, got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s[%d]: %v want %v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestBuildStepsAptAliasAndShareDir(t *testing.T) {
	h := "/home/u"
	steps, err := uninstall.BuildSteps([]catalog.ID{"fd", "neovim"}, "apt", 1000, uninstall.Probe{Home: h, Exists: set("/usr/bin/fdfind", h+"/.local/bin/nvim")})
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Title != "remove fd" || len(steps[0].Links) != 1 || steps[0].Links[0] != h+"/.local/bin/fd" {
		t.Fatalf("fd %+v", steps[0])
	}
	if steps[1].Title != "release-remove neovim" || steps[1].ShareDir != h+"/.local/share/nvim" {
		t.Fatalf("nvim %+v", steps[1])
	}
}

// fullHome builds a home that looks like the installer ran there.
func fullHome(t *testing.T) (home, repo string) {
	home = t.TempDir()
	repo = t.TempDir()
	put(t, filepath.Join(repo, "starship", "starship.toml"), "theme\n")
	put(t, filepath.Join(repo, "zellij", "config.kdl"), "cfg\n")
	put(t, filepath.Join(repo, "zellij", "layouts", "default.kdl"), "layout\n")
	put(t, filepath.Join(repo, "shells", "zsh", "zshrc"), "# tpl\n")
	put(t, filepath.Join(home, ".local", "bin", "starship"), "bin")
	put(t, filepath.Join(home, ".config", "starship.toml"), "theme\n")
	put(t, filepath.Join(home, ".config", "zellij", "config.kdl"), "edited\n")
	put(t, filepath.Join(home, ".config", "zellij", "layouts", "default.kdl"), "layout\n")
	for _, sh := range shellrc.Shells {
		path := shellrc.RcPath(home, sh)
		put(t, path, "# user "+string(sh)+"\n")
		if _, err := shellrc.EnsureFile(path, home, sh, []shellrc.BlockID{shellrc.BlockLocalBin, shellrc.BlockStarship, shellrc.BlockFzf}); err != nil {
			t.Fatal(err)
		}
	}
	return home, repo
}

func TestRunRemovesBinaryRcAndConfig(t *testing.T) {
	home, repo := fullHome(t)
	put(t, filepath.Join(home, ".local", "bin", "zellij"), "bin")
	steps, err := uninstall.BuildSteps([]catalog.ID{"starship", "zellij"}, "dnf", 1000, uninstall.Probe{Home: home, Exists: func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{}
	r := &uninstall.Runner{Exec: ex, Home: home, Repo: repo, Now: func() time.Time { return time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC) }}
	results, code := r.Run(context.Background(), steps)
	if code != 0 || len(results) != 2 {
		t.Fatalf("code %d %+v", code, results)
	}
	for _, res := range results {
		if res.Status != uninstall.StatusRemoved {
			t.Fatalf("%+v", res)
		}
	}
	if len(ex.calls) != 0 {
		t.Fatalf("release removal must not exec: %v", ex.calls)
	}
	gone := []string{".local/bin/starship", ".local/bin/zellij", ".config/starship.toml", ".config/zellij/config.kdl", ".config/zellij/layouts"}
	for _, rel := range gone {
		if _, err := os.Stat(filepath.Join(home, rel)); !os.IsNotExist(err) {
			t.Errorf("%s still there", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "zellij", "config.kdl.bak.20261117T000000Z")); err != nil {
		t.Errorf("edited zellij config not backed up: %v", err)
	}
	if !strings.Contains(results[1].Note, "已備份") {
		t.Errorf("note %q", results[1].Note)
	}
	for _, sh := range shellrc.Shells {
		body, _ := os.ReadFile(shellrc.RcPath(home, sh))
		s := string(body)
		if strings.Contains(s, "starship") {
			t.Errorf("%s still has starship:\n%s", sh, s)
		}
		if !strings.Contains(s, shellrc.Begin(shellrc.BlockFzf)) || !strings.Contains(s, shellrc.Begin(shellrc.BlockLocalBin)) {
			t.Errorf("%s lost other blocks:\n%s", sh, s)
		}
		if !strings.HasPrefix(s, "# user "+string(sh)+"\n") {
			t.Errorf("%s lost user lines", sh)
		}
	}
}

func TestRunPackageFailureKeepsConfig(t *testing.T) {
	home, repo := fullHome(t)
	steps, err := uninstall.BuildSteps([]catalog.ID{"fzf"}, "dnf", 1000, uninstall.Probe{Home: home, Exists: set("/usr/bin/fzf")})
	if err != nil {
		t.Fatal(err)
	}
	ex := &fakeExec{fn: func([]string) (string, int, error) { return "Error: boom", 1, nil }}
	r := &uninstall.Runner{Exec: ex, Home: home, Repo: repo}
	results, code := r.Run(context.Background(), steps)
	if code != 1 || len(results) != 1 || results[0].Status != "failed" || results[0].ErrText == "" {
		t.Fatalf("code %d %+v", code, results)
	}
	if len(ex.calls) != 1 || strings.Join(ex.calls[0], " ") != "sudo dnf remove -y fzf" {
		t.Fatalf("calls %v", ex.calls)
	}
	body, _ := os.ReadFile(shellrc.RcPath(home, shellrc.Zsh))
	if !strings.Contains(string(body), shellrc.Begin(shellrc.BlockFzf)) {
		t.Fatal("rc block removed although the package removal failed")
	}
}

func TestRunCancel(t *testing.T) {
	home, repo := fullHome(t)
	steps, _ := uninstall.BuildSteps([]catalog.ID{"fzf", "jq"}, "dnf", 1000, uninstall.Probe{Home: home, Exists: set("/usr/bin/fzf", "/usr/bin/jq")})
	ex := &fakeExec{fn: func([]string) (string, int, error) { return "", 130, nil }}
	r := &uninstall.Runner{Exec: ex, Home: home, Repo: repo}
	_, code := r.Run(context.Background(), steps)
	if code != 1 || len(ex.calls) != 1 {
		t.Fatalf("code %d calls %v", code, ex.calls)
	}
}

func TestRunReleaseTreeKeepsUserData(t *testing.T) {
	home := t.TempDir()
	share := filepath.Join(home, ".local", "share", "nvim")
	top := filepath.Join(share, "nvim-linux-x86_64")
	put(t, filepath.Join(top, "bin", "nvim"), "bin")
	put(t, filepath.Join(share, "lazy", "plugin.lua"), "user data")
	bin := filepath.Join(home, ".local", "bin", "nvim")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	if err := os.Symlink(filepath.Join(top, "bin", "nvim"), bin); err != nil {
		t.Fatal(err)
	}
	steps, err := uninstall.BuildSteps([]catalog.ID{"neovim"}, "dnf", 1000, uninstall.Probe{Home: home, Exists: set(bin)})
	if err != nil {
		t.Fatal(err)
	}
	r := &uninstall.Runner{Home: home}
	if results, code := r.Run(context.Background(), steps); code != 0 {
		t.Fatalf("%d %+v", code, results)
	}
	if _, err := os.Lstat(bin); !os.IsNotExist(err) {
		t.Error("bin kept")
	}
	if _, err := os.Stat(top); !os.IsNotExist(err) {
		t.Error("tree kept")
	}
	if _, err := os.Stat(filepath.Join(share, "lazy", "plugin.lua")); err != nil {
		t.Error("user data under ~/.local/share/nvim deleted")
	}
}

func TestRunGoAsRootAndAptAlias(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "go", "bin", "go"), "go")
	home := t.TempDir()
	steps, err := uninstall.BuildSteps([]catalog.ID{"go"}, "dnf", 0, uninstall.Probe{Home: home, GoRoot: root, Exists: set(filepath.Join(root, "go", "bin", "go"))})
	if err != nil {
		t.Fatal(err)
	}
	r := &uninstall.Runner{Home: home}
	if results, code := r.Run(context.Background(), steps); code != 0 {
		t.Fatalf("%d %+v", code, results)
	}
	if _, err := os.Stat(filepath.Join(root, "go")); !os.IsNotExist(err) {
		t.Fatal("go dir kept")
	}

	link := filepath.Join(home, ".local", "bin", "fd")
	os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink("/usr/bin/fdfind", link); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(home, ".local", "bin", "bat")
	put(t, other, "real bat, not an alias")
	steps, _ = uninstall.BuildSteps([]catalog.ID{"fd", "bat"}, "apt", 0, uninstall.Probe{Home: home, Exists: set("/usr/bin/fdfind", "/usr/bin/batcat")})
	ex := &fakeExec{}
	r = &uninstall.Runner{Exec: ex, Home: home}
	if results, code := r.Run(context.Background(), steps); code != 0 {
		t.Fatalf("%d %+v", code, results)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Error("fd alias kept")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("non-alias file removed")
	}
}

func TestRunRejectsTamperedSteps(t *testing.T) {
	home := t.TempDir()
	bad := []uninstall.Step{
		{ItemID: "go", Kind: uninstall.KindGo, GoDir: "/"},
		{ItemID: "go", Kind: uninstall.KindGo, GoDir: "/usr/local/go", Argv: []string{"sudo", "rm", "-rf", "/"}},
		{ItemID: "rust", Kind: uninstall.KindRustup, Argv: []string{"rm", "-rf", "/"}},
		{ItemID: "jq", Kind: uninstall.KindPackage, Argv: []string{"rm", "-rf", "/"}},
		{ItemID: "eza", Kind: uninstall.KindRelease, Bin: "/etc/passwd"},
	}
	for _, st := range bad {
		ex := &fakeExec{}
		r := &uninstall.Runner{Exec: ex, Home: home}
		results, code := r.Run(context.Background(), []uninstall.Step{st})
		if code != 1 || results[0].Status != "failed" || len(ex.calls) != 0 {
			t.Errorf("%+v ran: %d %+v %v", st, code, results, ex.calls)
		}
	}
}

func TestRunRemovesZigTarball(t *testing.T) {
	home := t.TempDir()
	share := filepath.Join(home, ".local", "share", "zig")
	top := filepath.Join(share, "zig-x86_64-linux-0.17.1")
	put(t, filepath.Join(top, "zig"), "zig")
	put(t, filepath.Join(top, "lib", "std", "std.zig"), "std")
	bin := filepath.Join(home, ".local", "bin", "zig")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	if err := os.Symlink(filepath.Join(top, "zig"), bin); err != nil {
		t.Fatal(err)
	}
	steps, err := uninstall.BuildSteps([]catalog.ID{"zig"}, "apt", 1000, uninstall.Probe{Home: home, Exists: set(bin)})
	if err != nil {
		t.Fatal(err)
	}
	if results, code := (&uninstall.Runner{Home: home}).Run(context.Background(), steps); code != 0 {
		t.Fatalf("%d %+v", code, results)
	}
	for _, p := range []string{bin, top, share} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s kept", p)
		}
	}
}
