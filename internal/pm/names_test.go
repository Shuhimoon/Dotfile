package pm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

// Literal 32×8 expectations. Do not derive these from names.go.
//
//	rel                     primary release
//	xcode                   xcode-clt
//	pre:paths               preinstalled, comma-separated OR
//	then:pkg:paths          package probes, comma is OR
//	then:pkg:paths>rel      same, release fallback
//	then:pkg:group;group    ProbeGroups, AND across groups
//	go:pkg:paths            package plus go-tarball, paths after the second colon
//	rust:pkg:paths          package plus rustup
//	name                    package, empty probes
var matrix = map[string][8]string{
	"git":        {"then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git", "then:git:/usr/bin/git,/opt/homebrew/bin/git,/usr/local/bin/git"},
	"curl":       {"then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/usr/bin/curl", "then:curl:/opt/homebrew/bin/curl,/usr/local/bin/curl"},
	"wget":       {"then:wget:/usr/bin/wget", "then:wget2-wget:/usr/bin/wget", "then:wget:/usr/bin/wget", "then:wget:/usr/bin/wget", "then:wget:/usr/bin/wget", "then:wget:/usr/bin/wget", "then:wget:/usr/bin/wget", "then:wget:/usr/bin/wget,/opt/homebrew/bin/wget,/usr/local/bin/wget"},
	"unzip":      {"then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip", "then:unzip:/usr/bin/unzip,/opt/homebrew/bin/unzip,/usr/local/bin/unzip"},
	"rsync":      {"then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync", "then:rsync:/usr/bin/rsync,/opt/homebrew/bin/rsync,/usr/local/bin/rsync"},
	"vim":        {"then:vim:/usr/bin/vim", "then:vim-enhanced:/usr/bin/vim", "then:vim-enhanced:/usr/bin/vim", "then:vim:/usr/bin/vim", "then:vim:/usr/bin/vim", "then:vim:/usr/bin/vim", "then:vim:/usr/bin/vim", "then:vim:/usr/bin/vim,/opt/homebrew/bin/vim,/usr/local/bin/vim"},
	"less":       row8("then:less:/usr/bin/less"),
	"net-tools":  {"then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "then:net-tools:/usr/bin/ifconfig,/usr/sbin/ifconfig", "pre:/sbin/ifconfig"},
	"whois":      row8("then:whois:/usr/bin/whois"),
	"dnsutils":   {"then:dnsutils:/usr/bin/dig", "then:bind-utils:/usr/bin/dig", "then:bind-utils:/usr/bin/dig", "then:bind:/usr/bin/dig", "then:bind-utils:/usr/bin/dig", "then:bind-tools:/usr/bin/dig", "then:bind-utils:/usr/bin/dig", "then:bind:/usr/bin/dig,/opt/homebrew/opt/bind/bin/dig,/usr/local/opt/bind/bin/dig,/opt/homebrew/bin/dig,/usr/local/bin/dig"},
	"traceroute": {"then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/bin/traceroute,/usr/sbin/traceroute", "then:traceroute:/usr/sbin/traceroute,/sbin/traceroute"},
	"zsh":        {"then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh", "then:zsh:/usr/bin/zsh,/opt/homebrew/bin/zsh,/usr/local/bin/zsh"},
	"starship":   {"then:starship:/usr/bin/starship>rel", "rel", "rel", "then:starship:/usr/bin/starship>rel", "then:starship:/usr/bin/starship>rel", "then:starship:/usr/bin/starship>rel", "then:starship:/usr/bin/starship>rel", "then:starship:/opt/homebrew/bin/starship,/usr/local/bin/starship,/usr/bin/starship>rel"},
	"eza":        {"then:eza:/usr/bin/eza>rel", "then:eza:/usr/bin/eza>rel", "rel", "then:eza:/usr/bin/eza>rel", "then:eza:/usr/bin/eza>rel", "then:eza:/usr/bin/eza>rel", "then:eza:/usr/bin/eza>rel", "then:eza:/opt/homebrew/bin/eza,/usr/local/bin/eza,/usr/bin/eza>rel"},
	"trash":      {"then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "then:trash-cli:/usr/bin/trash", "trash-cli"},
	"ripgrep":    {"then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/usr/bin/rg>rel", "then:ripgrep:/opt/homebrew/bin/rg,/usr/local/bin/rg,/usr/bin/rg>rel"},
	"fd":         {"then:fd-find:/usr/bin/fdfind>rel", "then:fd-find:/usr/bin/fd>rel", "then:fd-find:/usr/bin/fd>rel", "then:fd:/usr/bin/fd>rel", "then:fd:/usr/bin/fd>rel", "then:fd:/usr/bin/fd>rel", "then:fd:/usr/bin/fd>rel", "then:fd:/opt/homebrew/bin/fd,/usr/local/bin/fd>rel"},
	"bat":        {"then:bat:/usr/bin/batcat>rel", "then:bat:/usr/share/bash-completion/completions/bat.bash>rel", "then:bat:/usr/share/bash-completion/completions/bat.bash>rel", "then:bat:/usr/bin/bat>rel", "then:bat:/usr/bin/bat>rel", "then:bat:/usr/bin/bat>rel", "then:bat:/usr/bin/bat>rel", "then:bat:/opt/homebrew/bin/bat,/usr/local/bin/bat>rel"},
	"jq":         {"then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/usr/bin/jq", "then:jq:/opt/homebrew/bin/jq,/usr/local/bin/jq,/usr/bin/jq"},
	"fzf":        {"then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/usr/bin/fzf>rel", "then:fzf:/opt/homebrew/bin/fzf,/usr/local/bin/fzf,/usr/bin/fzf>rel"},
	"neovim":     {"then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/usr/bin/nvim>rel", "then:neovim:/opt/homebrew/bin/nvim,/usr/local/bin/nvim,/usr/bin/nvim>rel"},
	"zellij":     {"rel", "rel", "rel", "then:zellij:/usr/bin/zellij>rel", "then:zellij:/usr/bin/zellij>rel", "then:zellij:/usr/bin/zellij>rel", "then:zellij:/usr/bin/zellij>rel", "then:zellij:/opt/homebrew/bin/zellij,/usr/local/bin/zellij,/usr/bin/zellij>rel"},
	"fastfetch":  {"then:fastfetch:/usr/bin/fastfetch>rel", "then:fastfetch:/usr/bin/fastfetch>rel", "rel", "then:fastfetch:/usr/bin/fastfetch>rel", "then:fastfetch:/usr/bin/fastfetch>rel", "then:fastfetch:/usr/bin/fastfetch>rel", "then:fastfetch:/usr/bin/fastfetch>rel", "then:fastfetch:/opt/homebrew/bin/fastfetch,/usr/local/bin/fastfetch,/usr/bin/fastfetch>rel"},
	"mitos":      row8("rel"),
	"go":         {"go:golang-go:/usr/bin/go", "go:golang:/usr/bin/go", "go:golang:/usr/bin/go", "go:go:/usr/bin/go", "go:go:/usr/bin/go", "go:go:/usr/bin/go", "go:go:/usr/bin/go", "go:go:/opt/homebrew/bin/go,/usr/local/bin/go"},
	"rust":       {"rust:rustc|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust:/usr/bin/rustc;/usr/bin/cargo", "rust:rust|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust|cargo:/usr/bin/rustc;/usr/bin/cargo", "rust:rust:/opt/homebrew/bin/rustc,/usr/local/bin/rustc;/opt/homebrew/bin/cargo,/usr/local/bin/cargo"},
	"python":     {"then:python3:/usr/bin/python3", "then:python3:/usr/bin/python3", "then:python3:/usr/bin/python3", "then:python:/usr/bin/python3", "then:python3:/usr/bin/python3", "then:python3:/usr/bin/python3", "then:python3:/usr/bin/python3", "then:python:/opt/homebrew/bin/python3,/usr/local/bin/python3"},
	"node":       {"then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:nodejs:/usr/bin/node", "then:node:/opt/homebrew/bin/node,/usr/local/bin/node,/usr/bin/node"},
	"make":       {"then:make:/usr/bin/make", "then:make:/usr/bin/make", "then:make:/usr/bin/make", "then:make:/usr/bin/make", "then:make:/usr/bin/make", "then:make:/usr/bin/make", "then:make:/usr/bin/make", "pre:/usr/bin/make"},
	"cmake":      {"then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/usr/bin/cmake", "then:cmake:/opt/homebrew/bin/cmake,/usr/local/bin/cmake,/usr/bin/cmake"},
	"compiler":   {"then:build-essential:/usr/bin/gcc;/usr/bin/g++", "then:gcc|gcc-c++:/usr/bin/gcc;/usr/bin/g++", "then:gcc|gcc-c++:/usr/bin/gcc;/usr/bin/g++", "then:base-devel:/usr/bin/gcc;/usr/bin/g++", "then:gcc|gcc-c++:/usr/bin/gcc;/usr/bin/g++", "then:build-base:/usr/bin/gcc;/usr/bin/g++", "then:base-devel:/usr/bin/gcc;/usr/bin/g++", "xcode"},
	"pkg-config": {"then:pkg-config:/usr/bin/pkg-config", "then:pkgconf-pkg-config:/usr/bin/pkg-config", "then:pkgconfig:/usr/bin/pkg-config", "then:pkgconf:/usr/bin/pkg-config", "then:pkg-config:/usr/bin/pkg-config", "then:pkgconf:/usr/bin/pkg-config", "then:pkg-config:/usr/bin/pkg-config", "then:pkgconf:/opt/homebrew/bin/pkg-config,/usr/local/bin/pkg-config"},
}

var releaseRepos = map[string]string{
	"starship":  "starship/starship",
	"eza":       "eza-community/eza",
	"ripgrep":   "BurntSushi/ripgrep",
	"fd":        "sharkdp/fd",
	"bat":       "sharkdp/bat",
	"fzf":       "junegunn/fzf",
	"neovim":    "neovim/neovim",
	"zellij":    "zellij-org/zellij",
	"fastfetch": "fastfetch-cli/fastfetch",
	"mitos":     "mitos-editor/mitos",
}

func row8(cell string) [8]string {
	var out [8]string
	for i := range out {
		out[i] = cell
	}
	return out
}

func TestResolvePackageNames(t *testing.T) {
	if len(matrix) != 32 {
		t.Fatalf("matrix len = %d", len(matrix))
	}
	mgrs := pm.Managers()
	if len(mgrs) != 8 {
		t.Fatalf("managers %v", mgrs)
	}
	for id, cells := range matrix {
		repo := releaseRepos[id]
		for i, mgr := range mgrs {
			p, err := pm.Resolve(catalog.ID(id), mgr)
			if err != nil {
				t.Fatalf("%s/%s: %v", id, mgr, err)
			}
			assertCell(t, id, mgr, cells[i], repo, p)
		}
	}

	// Spot checks called out by the spec, so a partial matrix edit still fails loudly.
	mustPkg(t, "wget", "dnf", "wget2-wget")
	mustPkg(t, "vim", "dnf", "vim-enhanced")
	mustPkg(t, "dnsutils", "dnf", "bind-utils")
	mustPkg(t, "fd", "apt", "fd-find")
	mustPkg(t, "trash", "dnf", "trash-cli")
	mustPkg(t, "trash", "apt", "trash-cli")
	mustPkg(t, "node", "brew", "node")
	mustPkg(t, "compiler", "apt", "build-essential")
	mustPkg(t, "compiler", "apk", "build-base")
	mustPkg(t, "pkg-config", "dnf", "pkgconf-pkg-config")

	makePlan, err := pm.Resolve("make", "brew")
	if err != nil || makePlan.Method != pm.MethodPreinstalled || !slices.Equal(makePlan.ProbeFiles, []string{"/usr/bin/make"}) {
		t.Fatalf("brew make = %+v %v", makePlan, err)
	}
	for _, mgr := range pm.Managers() {
		lessPlan, err := pm.Resolve("less", mgr)
		if err != nil || lessPlan.Method != pm.MethodPackage || !slices.Equal(lessPlan.Packages, []string{"less"}) || !slices.Equal(lessPlan.ProbeFiles, []string{"/usr/bin/less"}) || len(lessPlan.ProbeGroups) != 0 {
			t.Fatalf("%s less = %+v %v", mgr, lessPlan, err)
		}
	}
	if makePlan.NeedsLocalBin || len(makePlan.Packages) != 0 || len(makePlan.ProbeGroups) != 0 {
		t.Fatalf("brew make should not install: %+v", makePlan)
	}
	banned := []string{"/usr/local/go", "/usr/local/go/bin/go"}
	for _, mgr := range pm.Managers() {
		p, err := pm.Resolve("go", mgr)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(p.ProbeFiles, func(f string) bool { return slices.Contains(banned, f) }) {
			t.Fatalf("%s go probes %v", mgr, p.ProbeFiles)
		}
		for _, g := range p.ProbeGroups {
			for _, f := range g {
				if slices.Contains(banned, f) {
					t.Fatalf("%s go group %v", mgr, g)
				}
			}
		}
		if p.Fallback == nil || len(p.Fallback.ProbeFiles) != 0 || len(p.Fallback.ProbeGroups) != 0 {
			t.Fatalf("%s go fallback %+v", mgr, p.Fallback)
		}
	}
}

func TestEveryCatalogIDHasPlan(t *testing.T) {
	ids := catalog.IDs()
	if len(ids) != 32 || len(matrix) != 32 {
		t.Fatalf("ids %d matrix %d", len(ids), len(matrix))
	}
	seen := map[catalog.ID]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate %s", id)
		}
		seen[id] = true
		if _, ok := matrix[string(id)]; !ok {
			t.Fatalf("matrix missing %s", id)
		}
		for _, mgr := range pm.Managers() {
			p, err := pm.Resolve(id, mgr)
			if err != nil {
				t.Fatalf("%s/%s: %v", id, mgr, err)
			}
			if p.ID != id {
				t.Fatalf("plan id %s", p.ID)
			}
			if p.Fallback != nil && p.Fallback.Fallback != nil {
				t.Fatalf("%s/%s fallback chained", id, mgr)
			}
		}
	}
	if _, err := pm.Resolve("not-a-tool", "dnf"); err == nil {
		t.Fatal("unknown id")
	}
	if _, err := pm.Resolve("git", "nix"); err == nil {
		t.Fatal("unknown manager")
	}
}

func mustPkg(t *testing.T, id, mgr, pkg string) {
	t.Helper()
	p, err := pm.Resolve(catalog.ID(id), mgr)
	if err != nil {
		t.Fatal(err)
	}
	if p.Method != pm.MethodPackage || !slices.Equal(p.Packages, []string{pkg}) {
		t.Fatalf("%s/%s = %+v", id, mgr, p)
	}
}

func assertCell(t *testing.T, id, mgr, cell, repo string, p pm.Plan) {
	t.Helper()
	wantRel := false
	if strings.HasSuffix(cell, ">rel") {
		wantRel = true
		cell = strings.TrimSuffix(cell, ">rel")
	}
	switch {
	case cell == "rel":
		if wantRel || p.Method != pm.MethodRelease || p.Fallback != nil || !p.NeedsLocalBin || p.ReleaseRepo != repo || len(p.Packages) != 0 || len(p.ProbeFiles) != 0 || len(p.ProbeGroups) != 0 {
			t.Fatalf("%s/%s rel = %+v", id, mgr, p)
		}
	case cell == "xcode":
		if wantRel || p.Method != pm.MethodXcodeCLT || p.Fallback != nil || p.NeedsLocalBin || len(p.Packages) != 0 || len(p.ProbeFiles) != 0 || len(p.ProbeGroups) != 0 {
			t.Fatalf("%s/%s xcode = %+v", id, mgr, p)
		}
	case strings.HasPrefix(cell, "pre:"):
		want := strings.Split(cell[len("pre:"):], ",")
		if wantRel || p.Method != pm.MethodPreinstalled || p.Fallback != nil || p.NeedsLocalBin || !slices.Equal(p.ProbeFiles, want) || len(p.ProbeGroups) != 0 {
			t.Fatalf("%s/%s pre = %+v", id, mgr, p)
		}
	case strings.HasPrefix(cell, "then:"):
		pkg, probes, ok := strings.Cut(cell[len("then:"):], ":")
		if !ok {
			t.Fatalf("%s/%s then missing colon %q", id, mgr, cell)
		}
		files, groups := expectProbes(probes)
		if p.Method != pm.MethodPackage || p.NeedsLocalBin || !slices.Equal(p.Packages, strings.Split(pkg, "|")) || !slices.Equal(p.ProbeFiles, files) {
			t.Fatalf("%s/%s then = %+v", id, mgr, p)
		}
		assertGroups(t, id, mgr, probes, groups, p.ProbeGroups)
		if !wantRel {
			if p.Fallback != nil {
				t.Fatalf("%s/%s then fallback %+v", id, mgr, p.Fallback)
			}
			return
		}
		if p.Fallback == nil || p.Fallback.Method != pm.MethodRelease || !p.Fallback.NeedsLocalBin || p.Fallback.ReleaseRepo != repo || len(p.Fallback.ProbeFiles) != 0 || len(p.Fallback.ProbeGroups) != 0 {
			t.Fatalf("%s/%s then rel %+v", id, mgr, p.Fallback)
		}
	case strings.HasPrefix(cell, "go:"), strings.HasPrefix(cell, "rust:"):
		prefix := "go:"
		method := pm.MethodGoTarball
		if strings.HasPrefix(cell, "rust:") {
			prefix = "rust:"
			method = pm.MethodRustup
		}
		pkg, probes, ok := strings.Cut(cell[len(prefix):], ":")
		if !ok {
			t.Fatalf("%s/%s missing probe colon %q", id, mgr, cell)
		}
		assertFallback(t, id, mgr, p, strings.Split(pkg, "|"), method, "", false)
		files, groups := expectProbes(probes)
		if !slices.Equal(p.ProbeFiles, files) {
			t.Fatalf("%s/%s probes %v", id, mgr, p.ProbeFiles)
		}
		assertGroups(t, id, mgr, probes, groups, p.ProbeGroups)
		if p.Fallback == nil || len(p.Fallback.ProbeFiles) != 0 || len(p.Fallback.ProbeGroups) != 0 {
			t.Fatalf("%s/%s fallback probes %+v", id, mgr, p.Fallback)
		}
	default:
		if wantRel || p.Method != pm.MethodPackage || p.Fallback != nil || p.NeedsLocalBin || len(p.ProbeFiles) != 0 || len(p.ProbeGroups) != 0 || !slices.Equal(p.Packages, strings.Split(cell, "|")) {
			t.Fatalf("%s/%s pkg %q = %+v", id, mgr, cell, p)
		}
	}
}

func assertGroups(t *testing.T, id, mgr, probes string, want, got [][]string) {
	t.Helper()
	if strings.Contains(probes, ";") {
		if !equalGroups(want, got) {
			t.Fatalf("%s/%s groups %v", id, mgr, got)
		}
		return
	}
	if len(got) != 0 {
		t.Fatalf("%s/%s unexpected groups %v", id, mgr, got)
	}
}

func expectProbes(paths string) (files []string, groups [][]string) {
	if strings.Contains(paths, ";") {
		for _, part := range strings.Split(paths, ";") {
			g := splitComma(part)
			groups = append(groups, g)
			files = append(files, g...)
		}
		return files, groups
	}
	return splitComma(paths), nil
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	if !strings.Contains(s, ",") {
		return []string{s}
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func equalGroups(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func assertFallback(t *testing.T, id, mgr string, p pm.Plan, pkgs []string, method pm.Method, repo string, local bool) {
	t.Helper()
	if p.Method != pm.MethodPackage || p.NeedsLocalBin || !slices.Equal(p.Packages, pkgs) {
		t.Fatalf("%s/%s primary = %+v", id, mgr, p)
	}
	if p.Fallback == nil || p.Fallback.Method != method || p.Fallback.Fallback != nil || p.Fallback.NeedsLocalBin != local {
		t.Fatalf("%s/%s fallback = %+v", id, mgr, p.Fallback)
	}
	if repo != "" && p.Fallback.ReleaseRepo != repo {
		t.Fatalf("%s/%s repo = %s", id, mgr, p.Fallback.ReleaseRepo)
	}
}
