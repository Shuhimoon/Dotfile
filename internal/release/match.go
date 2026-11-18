package release

import (
	"fmt"
	"strings"
)

type AssetQuery struct {
	GOOS    string
	GOARCH  string
	Libc    string // gnu or musl
	UnameM  string
	Version string
}

type Layout int

const (
	LayoutBinDirect Layout = iota
	LayoutBinInTopDir
	LayoutTree
)

type Spec struct {
	Repo        string
	BinaryName  string
	Layout      Layout
	ShareSubdir string
	SymlinkRel  string
}

func SpecFor(repo string) (Spec, bool) {
	s, ok := specs[repo]
	return s, ok
}

var specs = map[string]Spec{
	"starship/starship":       {Repo: "starship/starship", BinaryName: "starship", Layout: LayoutBinDirect},
	"eza-community/eza":       {Repo: "eza-community/eza", BinaryName: "eza", Layout: LayoutBinDirect},
	"zellij-org/zellij":       {Repo: "zellij-org/zellij", BinaryName: "zellij", Layout: LayoutBinDirect},
	"fastfetch-cli/fastfetch": {Repo: "fastfetch-cli/fastfetch", BinaryName: "fastfetch", Layout: LayoutTree, ShareSubdir: "fastfetch", SymlinkRel: "usr/bin/fastfetch"},
	"BurntSushi/ripgrep":      {Repo: "BurntSushi/ripgrep", BinaryName: "rg", Layout: LayoutBinInTopDir},
	"sharkdp/fd":              {Repo: "sharkdp/fd", BinaryName: "fd", Layout: LayoutBinInTopDir},
	"sharkdp/bat":             {Repo: "sharkdp/bat", BinaryName: "bat", Layout: LayoutBinInTopDir},
	"junegunn/fzf":            {Repo: "junegunn/fzf", BinaryName: "fzf", Layout: LayoutBinDirect},
	"neovim/neovim":           {Repo: "neovim/neovim", BinaryName: "nvim", Layout: LayoutTree, ShareSubdir: "nvim", SymlinkRel: "bin/nvim"},
	"mitos-editor/mitos":      {Repo: "mitos-editor/mitos", BinaryName: "ms", Layout: LayoutTree, ShareSubdir: "mitos", SymlinkRel: "ms"},
	ZigRepo:                   {Repo: ZigRepo, BinaryName: "zig", Layout: LayoutTree, ShareSubdir: "zig", SymlinkRel: "zig"},
}

func unsupported(goarch string) error {
	return fmt.Errorf("不支援的 GOARCH=%s：此安裝方法沒有對應的上游資產或 triple", goarch)
}

func Match(repo string, names []string, q AssetQuery) (string, error) {
	switch repo {
	case "starship/starship":
		return MatchStarship(names, q)
	case "eza-community/eza":
		return MatchEza(names, q)
	case "zellij-org/zellij":
		return MatchZellij(names, q)
	case "fastfetch-cli/fastfetch":
		return MatchFastfetch(names, q)
	case "BurntSushi/ripgrep":
		return MatchRipgrep(names, q)
	case "sharkdp/fd":
		return MatchFd(names, q)
	case "sharkdp/bat":
		return MatchBat(names, q)
	case "junegunn/fzf":
		return MatchFzf(names, q)
	case "neovim/neovim":
		return MatchNeovim(names, q)
	case "mitos-editor/mitos":
		return MatchMitos(names, q)
	default:
		return "", fmt.Errorf("未知的 repo %s", repo)
	}
}

func MatchStarship(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "", false, false)
}

func MatchEza(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "eza", false, false)
}

func MatchZellij(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "", true, false)
}

func MatchRipgrep(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "ripgrep-", false, true)
}

func MatchFd(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "fd-", false, false)
}

func MatchBat(names []string, q AssetQuery) (string, error) {
	return matchTriple(names, q, "bat-", false, false)
}

func MatchFastfetch(names []string, q AssetQuery) (string, error) {
	name, err := fastfetchName(q)
	if err != nil {
		return "", err
	}
	return exact(names, name)
}

func MatchNeovim(names []string, q AssetQuery) (string, error) {
	name, err := neovimName(q)
	if err != nil {
		return "", err
	}
	return exact(names, name)
}

func MatchMitos(names []string, q AssetQuery) (string, error) {
	name, err := mitosName(q)
	if err != nil {
		return "", err
	}
	return exact(names, name)
}

func MatchFzf(names []string, q AssetQuery) (string, error) {
	token, err := fzfToken(q)
	if err != nil {
		return "", err
	}
	suffix := "-" + token + ".tar.gz"
	var cands []string
	for _, n := range names {
		if strings.HasPrefix(n, "fzf-") && strings.HasSuffix(n, suffix) {
			cands = append(cands, n)
		}
	}
	return one(cands, names)
}

func fastfetchName(q AssetQuery) (string, error) {
	switch q.GOOS {
	case "darwin":
		if q.GOARCH == "amd64" || q.GOARCH == "arm64" {
			return "fastfetch-macos-universal.tar.gz", nil
		}
	case "linux":
		switch q.GOARCH {
		case "amd64":
			return "fastfetch-linux-amd64.tar.gz", nil
		case "arm64":
			return "fastfetch-linux-aarch64.tar.gz", nil
		case "386":
			return "fastfetch-linux-i686.tar.gz", nil
		case "arm":
			switch q.UnameM {
			case "armv6l":
				return "fastfetch-linux-armv6l.tar.gz", nil
			case "armv7l":
				return "fastfetch-linux-armv7l.tar.gz", nil
			default:
				return "", unsupported(q.GOARCH)
			}
		}
	}
	return "", unsupported(q.GOARCH)
}

func neovimName(q AssetQuery) (string, error) {
	switch q.GOOS + "/" + q.GOARCH {
	case "linux/amd64":
		return "nvim-linux-x86_64.tar.gz", nil
	case "linux/arm64":
		return "nvim-linux-arm64.tar.gz", nil
	case "darwin/amd64":
		return "nvim-macos-x86_64.tar.gz", nil
	case "darwin/arm64":
		return "nvim-macos-arm64.tar.gz", nil
	default:
		return "", unsupported(q.GOARCH)
	}
}

func mitosName(q AssetQuery) (string, error) {
	if q.Version == "" {
		return "", fmt.Errorf("mitos 缺少 version")
	}
	var mid string
	switch q.GOOS + "/" + q.GOARCH {
	case "linux/amd64":
		mid = "x86_64-linux"
	case "linux/arm64":
		mid = "aarch64-linux"
	case "darwin/amd64":
		mid = "x86_64-macos"
	case "darwin/arm64":
		mid = "aarch64-macos"
	default:
		return "", unsupported(q.GOARCH)
	}
	return "mitos-v" + q.Version + "-" + mid + ".tar.xz", nil
}

func fzfToken(q AssetQuery) (string, error) {
	switch q.GOOS {
	case "linux":
		switch q.GOARCH {
		case "amd64":
			return "linux_amd64", nil
		case "arm64":
			return "linux_arm64", nil
		case "arm":
			switch q.UnameM {
			case "armv5l":
				return "linux_armv5", nil
			case "armv6l":
				return "linux_armv6", nil
			case "armv7l":
				return "linux_armv7", nil
			default:
				return "", fmt.Errorf("不支援的 GOARCH=arm：uname-m 不足以選擇 linux_armv5、linux_armv6 或 linux_armv7")
			}
		case "loong64":
			return "linux_loong64", nil
		case "ppc64le":
			return "linux_ppc64le", nil
		case "s390x":
			return "linux_s390x", nil
		case "riscv64":
			return "linux_riscv64", nil
		default:
			return "", unsupported(q.GOARCH)
		}
	case "darwin":
		switch q.GOARCH {
		case "amd64":
			return "darwin_amd64", nil
		case "arm64":
			return "darwin_arm64", nil
		default:
			return "", unsupported(q.GOARCH)
		}
	default:
		return "", unsupported(q.GOARCH)
	}
}

func matchTriple(names []string, q AssetQuery, prefix string, muslIfNoGnu, linuxPreferMusl bool) (string, error) {
	arch, err := archToken(q.GOARCH)
	if err != nil {
		return "", err
	}
	if q.GOOS != "linux" && q.GOOS != "darwin" {
		return "", unsupported(q.GOARCH)
	}
	osTok := "apple-darwin"
	if q.GOOS == "linux" {
		osTok = "unknown-linux-gnu"
		if q.Libc == "musl" || linuxPreferMusl {
			osTok = "unknown-linux-musl"
		}
	}
	cands := filterTriple(names, prefix, arch, osTok)
	if len(cands) == 0 && q.GOOS == "linux" && muslIfNoGnu && osTok == "unknown-linux-gnu" {
		cands = filterTriple(names, prefix, arch, "unknown-linux-musl")
	}
	if len(cands) == 0 && q.GOOS == "linux" && linuxPreferMusl && osTok == "unknown-linux-musl" {
		cands = filterTriple(names, prefix, arch, "unknown-linux-gnu")
	}
	return one(dropVariants(cands), names)
}

func filterTriple(names []string, prefix, arch, osTok string) []string {
	var out []string
	for _, n := range names {
		if strings.Contains(n, ".sha256") || !strings.HasSuffix(n, ".tar.gz") {
			continue
		}
		if !prefixOK(n, prefix) || !containsArch(n, arch) || !strings.Contains(n, osTok) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func prefixOK(name, prefix string) bool {
	if prefix == "" {
		return true
	}
	if prefix == "eza" {
		return strings.HasPrefix(name, "eza_") || strings.HasPrefix(name, "eza-")
	}
	return strings.HasPrefix(name, prefix)
}

func archToken(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	case "386":
		return "i686", nil
	case "arm":
		return "arm", nil
	default:
		return "", unsupported(goarch)
	}
}

func containsArch(name, arch string) bool {
	if arch != "arm" {
		return strings.Contains(name, arch)
	}
	for _, b := range []string{"-arm-", "_arm-", "-arm.", "_arm."} {
		if strings.Contains(name, b) {
			return true
		}
	}
	return strings.HasSuffix(name, "-arm") || strings.HasSuffix(name, "_arm")
}

func exact(names []string, want string) (string, error) {
	var cands []string
	for _, n := range names {
		if n == want {
			cands = append(cands, n)
		}
	}
	return one(cands, names)
}

func one(cands, all []string) (string, error) {
	switch len(cands) {
	case 1:
		return cands[0], nil
	case 0:
		return "", fmt.Errorf("找不到資產：%s", list30(all))
	default:
		return "", fmt.Errorf("多個資產符合：%s", strings.Join(cands, ", "))
	}
}

func list30(names []string) string {
	if len(names) > 30 {
		names = names[:30]
	}
	return strings.Join(names, ", ")
}

// dropVariants removes feature-reduced builds (zellij ships
// zellij-no-web-<triple>.tar.gz next to zellij-<triple>.tar.gz) when the full
// build is also there, so the pick is not ambiguous.
func dropVariants(cands []string) []string {
	if len(cands) < 2 {
		return cands
	}
	var out []string
	for _, n := range cands {
		if !strings.Contains(n, "-no-web-") {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return cands
	}
	return out
}
