// Package uninstall removes tools the installer put on the machine: the
// package or binary, the managed shell rc blocks, and the deployed config.
// The 基礎 (basics) category is never removable.
package uninstall

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

type Kind string

const (
	KindPackage Kind = "package"
	KindRelease Kind = "release"
	KindGo      Kind = "go-tarball"
	KindRustup  Kind = "rustup"
	KindShell   Kind = "shell-rc"
	KindConfig  Kind = "config"
	// KindSSH drops the managed "dotfiles:github" block from ~/.ssh/config
	// when git is removed. Key files are kept.
	KindSSH Kind = "ssh-config"
)

// LockedCategory is the catalog category that can never be removed.
const LockedCategory = "basics"

// PythonWarning is shown on the progress page when python is removed (it does not block).
const PythonWarning = "警告：Fedora 的 dnf（以及許多系統工具）需要 python3 才能執行，移除 python 可能讓套件管理器壞掉。"

type Step struct {
	ItemID  catalog.ID
	Title   string
	Kind    Kind
	Manager string
	// Argv is the command for package, rustup, and a non-root go-tarball removal.
	Argv []string
	// Bin is ~/.local/bin/<binary> for a release install.
	Bin string
	// ShareDir is ~/.local/share/<subdir> for a release that unpacked a tree.
	ShareDir string
	// GoDir is the Go tarball root (/usr/local/go) removed in-process as root.
	GoDir string
	// Links are Debian fd/bat command-name symlinks dropped after the package.
	Links  []string
	Blocks []shellrc.BlockID
}

// Probe is how BuildSteps looks at the machine. Exists defaults to false.
type Probe struct {
	Home   string
	GoRoot string // parent of go/, default /usr/local
	Exists func(string) bool
}

func (p Probe) exists(path string) bool {
	return p.Exists != nil && p.Exists(path)
}

func (p Probe) goDir() string {
	root := p.GoRoot
	if root == "" {
		root = "/usr/local"
	}
	return filepath.Join(root, "go")
}

// Removable is false for unknown ids and for every item in the basics category.
func Removable(id catalog.ID) bool {
	for _, c := range catalog.All() {
		for _, it := range c.Items {
			if it.ID == id {
				return c.ID != LockedCategory
			}
		}
	}
	return false
}

// NeedsWarning reports the warning text shown while an item is removed.
func NeedsWarning(id catalog.ID) (string, bool) {
	if id == "python" {
		return PythonWarning, true
	}
	return "", false
}

// Supported is false when manager installs id in a way this tool cannot undo
// (Xcode Command Line Tools, or a preinstalled system file).
func Supported(id catalog.ID, manager string) bool {
	p, err := pm.Resolve(id, manager)
	if err != nil {
		return false
	}
	switch p.Method {
	case pm.MethodXcodeCLT, pm.MethodPreinstalled:
		return false
	}
	return true
}

// BlocksOf is the managed rc blocks owned by id. The shared local-bin block is
// kept: other release tools still need ~/.local/bin on PATH.
func BlocksOf(id catalog.ID) []shellrc.BlockID {
	switch id {
	case "starship":
		return []shellrc.BlockID{shellrc.BlockStarship}
	case "fzf":
		return []shellrc.BlockID{shellrc.BlockFzf}
	case "rust":
		return []shellrc.BlockID{shellrc.BlockRust}
	case "go":
		return []shellrc.BlockID{shellrc.BlockGo}
	}
	return nil
}

// BuildSteps plans the removal of ids, in catalog order. For each item it
// removes whatever install method left files behind (package, release binary,
// Go tarball, rustup), then its rc blocks, then its config files.
func BuildSteps(ids []catalog.ID, manager string, euid int, pr Probe) ([]Step, error) {
	ids = append([]catalog.ID(nil), ids...)
	sort.SliceStable(ids, func(i, j int) bool { return catalogIndex(ids[i]) < catalogIndex(ids[j]) })
	seen := map[catalog.ID]bool{}
	var steps []Step
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if !Removable(id) {
			return nil, fmt.Errorf("%s 屬於基礎類，不可移除", id)
		}
		if !Supported(id, manager) {
			return nil, fmt.Errorf("%s 由系統提供，無法移除", id)
		}
		plan, err := pm.Resolve(id, manager)
		if err != nil {
			return nil, err
		}
		item, err := binarySteps(plan, manager, euid, pr)
		if err != nil {
			return nil, err
		}
		steps = append(steps, item...)
		if b := BlocksOf(id); len(b) > 0 {
			steps = append(steps, Step{ItemID: id, Title: "shell-rc " + string(id), Kind: KindShell, Blocks: b})
		}
		if configcopy.HasConfig(string(id)) {
			steps = append(steps, Step{ItemID: id, Title: "config " + string(id), Kind: KindConfig})
		}
		if id == "git" {
			steps = append(steps, Step{ItemID: id, Title: "ssh-config git", Kind: KindSSH})
		}
	}
	return steps, nil
}

func binarySteps(plan pm.Plan, manager string, euid int, pr Probe) ([]Step, error) {
	id := plan.ID
	methods := []pm.Method{plan.Method}
	repo := plan.ReleaseRepo
	if plan.Fallback != nil {
		methods = append(methods, plan.Fallback.Method)
		if repo == "" {
			repo = plan.Fallback.ReleaseRepo
		}
	}
	var out []Step
	var pkgStep *Step
	for _, m := range methods {
		switch m {
		case pm.MethodPackage:
			argv, err := pm.BuildRemove(manager, plan.Packages)
			if err != nil {
				return nil, err
			}
			argv = pm.WithPrivilege(argv, pm.NeedsPrivilege(manager), euid)
			if err := pm.ValidateArgv(argv); err != nil {
				return nil, err
			}
			st := Step{ItemID: id, Title: "remove " + string(id), Kind: KindPackage, Manager: manager, Argv: argv}
			if manager == "apt" && pr.Home != "" && (id == "fd" || id == "bat") {
				st.Links = []string{filepath.Join(pr.Home, ".local", "bin", string(id))}
			}
			if install.AlreadySatisfied(pm.MethodPackage, plan.ProbeFiles, plan.ProbeGroups, pr.exists, false) {
				out = append(out, st)
			} else {
				pkgStep = &st
			}
		case pm.MethodRelease:
			spec, ok := release.SpecFor(repo)
			if !ok || pr.Home == "" {
				continue
			}
			bin := filepath.Join(pr.Home, ".local", "bin", spec.BinaryName)
			if !pr.exists(bin) {
				continue
			}
			st := Step{ItemID: id, Title: "release-remove " + string(id), Kind: KindRelease, Bin: bin}
			if spec.ShareSubdir != "" {
				st.ShareDir = filepath.Join(pr.Home, ".local", "share", spec.ShareSubdir)
			}
			out = append(out, st)
		case pm.MethodGoTarball:
			dir := pr.goDir()
			if !pr.exists(filepath.Join(dir, "bin", "go")) {
				continue
			}
			st := Step{ItemID: id, Title: "go-tarball-remove " + string(id), Kind: KindGo, GoDir: dir}
			if euid != 0 {
				st.Argv = []string{"sudo", "rm", "-rf", dir}
			}
			out = append(out, st)
		case pm.MethodRustup:
			if pr.Home == "" {
				continue
			}
			rustup := filepath.Join(pr.Home, ".cargo", "bin", "rustup")
			if !pr.exists(rustup) {
				continue
			}
			out = append(out, Step{ItemID: id, Title: "rustup-remove " + string(id), Kind: KindRustup, Argv: []string{rustup, "self", "uninstall", "-y"}})
		}
	}
	// Nothing found on disk: still ask the package manager, which reports
	// "not installed" itself, rather than silently doing nothing.
	if len(out) == 0 && pkgStep != nil {
		out = append(out, *pkgStep)
	}
	return out, nil
}

func catalogIndex(id catalog.ID) int {
	n := 0
	for _, c := range catalog.All() {
		for _, it := range c.Items {
			if it.ID == id {
				return n
			}
			n++
		}
	}
	return n + 1
}
