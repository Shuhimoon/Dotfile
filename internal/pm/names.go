package pm

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shuhi/dotfiles/internal/catalog"
)

type Method string

const (
	MethodPackage      Method = "package"
	MethodRelease      Method = "release"
	MethodPreinstalled Method = "preinstalled"
	MethodXcodeCLT     Method = "xcode-clt"
	MethodRustup       Method = "rustup"
	MethodGoTarball    Method = "go-tarball"
)

type Plan struct {
	ID            catalog.ID
	Method        Method
	Packages      []string
	Fallback      *Plan
	ReleaseRepo   string
	ProbeFiles    []string
	ProbeGroups   [][]string
	NeedsLocalBin bool
}

var pkgNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.-]*$`)

var mgrIndex = map[string]int{
	"apt": 0, "dnf": 1, "yum": 2, "pacman": 3, "zypper": 4, "apk": 5, "xbps": 6, "brew": 7,
}

type itemSpec struct {
	cell [8]string
	repo string
	fb   string
}

func base(pkg string) [8]string {
	var c [8]string
	for i := range c {
		c[i] = pkg
	}
	return c
}

func with(c [8]string, pairs ...string) [8]string {
	for i := 0; i+1 < len(pairs); i += 2 {
		c[mgrIndex[pairs[i]]] = pairs[i+1]
	}
	return c
}

func atThen(pkg, paths string) string {
	return "@then:" + pkg + ":" + paths
}

func specs() map[catalog.ID]itemSpec {
	linuxCC := "/usr/bin/gcc;/usr/bin/g++"
	linuxRust := "/usr/bin/rustc;/usr/bin/cargo"
	brewRust := "/opt/homebrew/bin/rustc,/usr/local/bin/rustc;/opt/homebrew/bin/cargo,/usr/local/bin/cargo"
	return map[catalog.ID]itemSpec{
		"git":   {cell: with(base(atThen("git", "/usr/bin/git")), "brew", atThen("git", "/usr/bin/git,/opt/homebrew/bin/git,/usr/local/bin/git"))},
		"curl":  {cell: with(base(atThen("curl", "/usr/bin/curl")), "brew", atThen("curl", "/opt/homebrew/bin/curl,/usr/local/bin/curl"))},
		"wget":  {cell: with(base(atThen("wget", "/usr/bin/wget")), "dnf", atThen("wget2-wget", "/usr/bin/wget"), "brew", atThen("wget", "/usr/bin/wget,/opt/homebrew/bin/wget,/usr/local/bin/wget"))},
		"unzip": {cell: with(base(atThen("unzip", "/usr/bin/unzip")), "brew", atThen("unzip", "/usr/bin/unzip,/opt/homebrew/bin/unzip,/usr/local/bin/unzip"))},
		"rsync": {cell: with(base(atThen("rsync", "/usr/bin/rsync")), "brew", atThen("rsync", "/usr/bin/rsync,/opt/homebrew/bin/rsync,/usr/local/bin/rsync"))},
		"vim": {cell: with(base(atThen("vim", "/usr/bin/vim")),
			"dnf", atThen("vim-enhanced", "/usr/bin/vim"),
			"yum", atThen("vim-enhanced", "/usr/bin/vim"),
			"brew", atThen("vim", "/usr/bin/vim,/opt/homebrew/bin/vim,/usr/local/bin/vim"))},
		"less":      {cell: base(atThen("less", "/usr/bin/less"))},
		"net-tools": {cell: with(base(atThen("net-tools", "/usr/bin/ifconfig,/usr/sbin/ifconfig")), "brew", "@pre:/sbin/ifconfig")},
		"whois":     {cell: base(atThen("whois", "/usr/bin/whois"))},
		"dnsutils": {cell: [8]string{
			atThen("dnsutils", "/usr/bin/dig"),
			atThen("bind-utils", "/usr/bin/dig"),
			atThen("bind-utils", "/usr/bin/dig"),
			atThen("bind", "/usr/bin/dig"),
			atThen("bind-utils", "/usr/bin/dig"),
			atThen("bind-tools", "/usr/bin/dig"),
			atThen("bind-utils", "/usr/bin/dig"),
			atThen("bind", "/usr/bin/dig,/opt/homebrew/opt/bind/bin/dig,/usr/local/opt/bind/bin/dig,/opt/homebrew/bin/dig,/usr/local/bin/dig"),
		}},
		"traceroute": {cell: with(base(atThen("traceroute", "/usr/bin/traceroute,/usr/sbin/traceroute")), "brew", atThen("traceroute", "/usr/sbin/traceroute,/sbin/traceroute"))},
		"zsh":        {cell: with(base(atThen("zsh", "/usr/bin/zsh")), "brew", atThen("zsh", "/usr/bin/zsh,/opt/homebrew/bin/zsh,/usr/local/bin/zsh"))},
		"starship": {cell: with(base(atThen("starship", "/usr/bin/starship")),
			"dnf", "@rel", "yum", "@rel",
			"brew", atThen("starship", "/opt/homebrew/bin/starship,/usr/local/bin/starship,/usr/bin/starship")),
			repo: "starship/starship", fb: "release"},
		"eza": {cell: with(base(atThen("eza", "/usr/bin/eza")),
			"yum", "@rel",
			"brew", atThen("eza", "/opt/homebrew/bin/eza,/usr/local/bin/eza,/usr/bin/eza")),
			repo: "eza-community/eza", fb: "release"},
		"trash":   {cell: with(base(atThen("trash-cli", "/usr/bin/trash")), "brew", "trash-cli")},
		"ripgrep": {cell: with(base(atThen("ripgrep", "/usr/bin/rg")), "brew", atThen("ripgrep", "/opt/homebrew/bin/rg,/usr/local/bin/rg,/usr/bin/rg")), repo: "BurntSushi/ripgrep", fb: "release"},
		"fd": {cell: [8]string{
			atThen("fd-find", "/usr/bin/fdfind"),
			atThen("fd-find", "/usr/bin/fd"),
			atThen("fd-find", "/usr/bin/fd"),
			atThen("fd", "/usr/bin/fd"),
			atThen("fd", "/usr/bin/fd"),
			atThen("fd", "/usr/bin/fd"),
			atThen("fd", "/usr/bin/fd"),
			atThen("fd", "/opt/homebrew/bin/fd,/usr/local/bin/fd"),
		}, repo: "sharkdp/fd", fb: "release"},
		"bat": {cell: [8]string{
			atThen("bat", "/usr/bin/batcat"),
			atThen("bat", "/usr/share/bash-completion/completions/bat.bash"),
			atThen("bat", "/usr/share/bash-completion/completions/bat.bash"),
			atThen("bat", "/usr/bin/bat"),
			atThen("bat", "/usr/bin/bat"),
			atThen("bat", "/usr/bin/bat"),
			atThen("bat", "/usr/bin/bat"),
			atThen("bat", "/opt/homebrew/bin/bat,/usr/local/bin/bat"),
		}, repo: "sharkdp/bat", fb: "release"},
		"jq":     {cell: with(base(atThen("jq", "/usr/bin/jq")), "brew", atThen("jq", "/opt/homebrew/bin/jq,/usr/local/bin/jq,/usr/bin/jq"))},
		"fzf":    {cell: with(base(atThen("fzf", "/usr/bin/fzf")), "brew", atThen("fzf", "/opt/homebrew/bin/fzf,/usr/local/bin/fzf,/usr/bin/fzf")), repo: "junegunn/fzf", fb: "release"},
		"neovim": {cell: with(base(atThen("neovim", "/usr/bin/nvim")), "brew", atThen("neovim", "/opt/homebrew/bin/nvim,/usr/local/bin/nvim,/usr/bin/nvim")), repo: "neovim/neovim", fb: "release"},
		"zellij": {cell: [8]string{
			"@rel", "@rel", "@rel",
			atThen("zellij", "/usr/bin/zellij"),
			atThen("zellij", "/usr/bin/zellij"),
			atThen("zellij", "/usr/bin/zellij"),
			atThen("zellij", "/usr/bin/zellij"),
			atThen("zellij", "/opt/homebrew/bin/zellij,/usr/local/bin/zellij,/usr/bin/zellij"),
		}, repo: "zellij-org/zellij", fb: "release"},
		"fastfetch": {cell: with(base(atThen("fastfetch", "/usr/bin/fastfetch")),
			"yum", "@rel",
			"brew", atThen("fastfetch", "/opt/homebrew/bin/fastfetch,/usr/local/bin/fastfetch,/usr/bin/fastfetch")),
			repo: "fastfetch-cli/fastfetch", fb: "release"},
		"mitos": {cell: base("@rel"), repo: "mitos-editor/mitos"},
		"go": {cell: [8]string{
			atThen("golang-go", "/usr/bin/go"),
			atThen("golang", "/usr/bin/go"),
			atThen("golang", "/usr/bin/go"),
			atThen("go", "/usr/bin/go"),
			atThen("go", "/usr/bin/go"),
			atThen("go", "/usr/bin/go"),
			atThen("go", "/usr/bin/go"),
			atThen("go", "/opt/homebrew/bin/go,/usr/local/bin/go"),
		}, fb: "go-tarball"},
		"rust": {cell: [8]string{
			atThen("rustc|cargo", linuxRust),
			atThen("rust|cargo", linuxRust),
			atThen("rust|cargo", linuxRust),
			atThen("rust", linuxRust),
			atThen("rust|cargo", linuxRust),
			atThen("rust|cargo", linuxRust),
			atThen("rust|cargo", linuxRust),
			atThen("rust", brewRust),
		}, fb: "rustup"},
		"python": {cell: [8]string{
			atThen("python3", "/usr/bin/python3"),
			atThen("python3", "/usr/bin/python3"),
			atThen("python3", "/usr/bin/python3"),
			atThen("python", "/usr/bin/python3"),
			atThen("python3", "/usr/bin/python3"),
			atThen("python3", "/usr/bin/python3"),
			atThen("python3", "/usr/bin/python3"),
			atThen("python", "/opt/homebrew/bin/python3,/usr/local/bin/python3"),
		}},
		"node":  {cell: with(base(atThen("nodejs", "/usr/bin/node")), "brew", atThen("node", "/opt/homebrew/bin/node,/usr/local/bin/node,/usr/bin/node"))},
		"make":  {cell: with(base(atThen("make", "/usr/bin/make")), "brew", "@pre:/usr/bin/make")},
		"cmake": {cell: with(base(atThen("cmake", "/usr/bin/cmake")), "brew", atThen("cmake", "/opt/homebrew/bin/cmake,/usr/local/bin/cmake,/usr/bin/cmake"))},
		"compiler": {cell: [8]string{
			atThen("build-essential", linuxCC),
			atThen("gcc|gcc-c++", linuxCC),
			atThen("gcc|gcc-c++", linuxCC),
			atThen("base-devel", linuxCC),
			atThen("gcc|gcc-c++", linuxCC),
			atThen("build-base", linuxCC),
			atThen("base-devel", linuxCC),
			"@xcode",
		}},
		"pkg-config": {cell: [8]string{
			atThen("pkg-config", "/usr/bin/pkg-config"),
			atThen("pkgconf-pkg-config", "/usr/bin/pkg-config"),
			atThen("pkgconfig", "/usr/bin/pkg-config"),
			atThen("pkgconf", "/usr/bin/pkg-config"),
			atThen("pkg-config", "/usr/bin/pkg-config"),
			atThen("pkgconf", "/usr/bin/pkg-config"),
			atThen("pkg-config", "/usr/bin/pkg-config"),
			atThen("pkgconf", "/opt/homebrew/bin/pkg-config,/usr/local/bin/pkg-config"),
		}},
	}
}

func Resolve(id catalog.ID, manager string) (Plan, error) {
	idx, ok := mgrIndex[manager]
	if !ok {
		return Plan{}, fmt.Errorf("未知的套件管理器 %s", manager)
	}
	spec, ok := specs()[id]
	if !ok {
		return Plan{}, fmt.Errorf("未知的 ID %s", id)
	}
	return parseCell(id, spec.cell[idx], spec.repo, spec.fb)
}

func parseCell(id catalog.ID, raw, repo, fb string) (Plan, error) {
	p := Plan{ID: id}
	switch {
	case raw == "@rel":
		if repo == "" {
			return Plan{}, fmt.Errorf("%s 缺少 release repo", id)
		}
		p.Method = MethodRelease
		p.ReleaseRepo = repo
		p.NeedsLocalBin = true
		return p, nil
	case raw == "@xcode":
		p.Method = MethodXcodeCLT
		return p, nil
	case strings.HasPrefix(raw, "@pre:"):
		p.Method = MethodPreinstalled
		p.ProbeFiles = splitList(strings.TrimPrefix(raw, "@pre:"))
		return p, nil
	case strings.HasPrefix(raw, "@then:"):
		rest := strings.TrimPrefix(raw, "@then:")
		pkg, probes, ok := strings.Cut(rest, ":")
		if !ok {
			return Plan{}, fmt.Errorf("無法解析 %s", raw)
		}
		p.Method = MethodPackage
		p.Packages = splitList(pkg)
		files, groups, err := splitProbes(probes)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", id, err)
		}
		p.ProbeFiles = files
		p.ProbeGroups = groups
	default:
		p.Method = MethodPackage
		p.Packages = splitList(raw)
	}
	if len(p.Packages) == 0 {
		return Plan{}, fmt.Errorf("%s 沒有套件名稱", id)
	}
	for _, name := range p.Packages {
		if !pkgNameRe.MatchString(name) {
			return Plan{}, fmt.Errorf("非法套件名稱 %s", name)
		}
	}
	switch fb {
	case "release":
		p.Fallback = &Plan{ID: id, Method: MethodRelease, ReleaseRepo: repo, NeedsLocalBin: true}
	case "go-tarball":
		p.Fallback = &Plan{ID: id, Method: MethodGoTarball}
	case "rustup":
		p.Fallback = &Plan{ID: id, Method: MethodRustup}
	}
	return p, nil
}

// splitProbes keeps a comma list as one OR group. A semicolon splits AND groups;
// the flat list is those groups in order and is not the satisfied decision.
func splitProbes(s string) ([]string, [][]string, error) {
	if strings.Contains(s, "|") {
		return nil, nil, fmt.Errorf("探測路徑不得包含 |")
	}
	if !strings.Contains(s, ";") {
		return splitList(s), nil, nil
	}
	parts := strings.Split(s, ";")
	groups := make([][]string, 0, len(parts))
	var flat []string
	for _, part := range parts {
		g := splitList(part)
		if len(g) == 0 {
			return nil, nil, fmt.Errorf("空的探測組")
		}
		groups = append(groups, g)
		flat = append(flat, g...)
	}
	return flat, groups, nil
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "|")
	if strings.Contains(s, ",") && !strings.Contains(s, "|") {
		parts = strings.Split(s, ",")
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
