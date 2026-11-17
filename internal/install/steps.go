package install

import (
	"context"
	"fmt"
	"sort"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/pm"
)

type Kind string

const (
	KindUpdate  Kind = "update"
	KindPackage Kind = "package"
	KindRelease Kind = "release"
	KindProbe   Kind = "probe"
	KindXcode   Kind = "xcode"
	KindGo      Kind = "go-tarball"
	KindRustup  Kind = "rustup"
	KindConfig  Kind = "config"
	KindLink    Kind = "link-bash"
)

type Step struct {
	ItemID      catalog.ID
	Title       string
	Argv        []string
	Hooks       []string
	Local       func(ctx context.Context) error
	Kind        Kind
	Manager     string
	Method      pm.Method
	Fallback    *Step
	ProbeFiles  []string
	ProbeGroups [][]string
	ReleaseRepo string
}

func BuildSteps(plans []Plan, euid int, home string) ([]Step, error) {
	plans = append([]Plan(nil), plans...)
	sort.SliceStable(plans, func(i, j int) bool {
		return catalogIndex(plans[i].ID) < catalogIndex(plans[j].ID)
	})
	if len(plans) == 0 {
		return nil, nil
	}
	var steps []Step
	if hasPackage(plans) {
		upd, err := pm.BuildUpdate(plans[0].Manager)
		if err != nil {
			return nil, err
		}
		if upd != nil {
			upd = pm.WithPrivilege(upd, pm.NeedsPrivilege(plans[0].Manager), euid)
			if err := pm.ValidateArgv(upd); err != nil {
				return nil, err
			}
			steps = append(steps, Step{
				Title:   "update",
				Argv:    upd,
				Kind:    KindUpdate,
				Manager: plans[0].Manager,
			})
		}
	}
	selected := map[catalog.ID]bool{}
	for _, p := range plans {
		selected[p.ID] = true
		st, err := itemStep(p, euid)
		if err != nil {
			return nil, err
		}
		steps = append(steps, st)
	}
	for _, cid := range configcopy.ToolIDs {
		id := catalog.ID(cid)
		if selected[id] {
			steps = append(steps, Step{
				ItemID:  id,
				Title:   "config " + string(id),
				Kind:    KindConfig,
				Method:  pm.MethodPackage,
				Manager: plans[0].Manager,
			})
		}
	}
	hooks := plannedHooks(plans)
	if len(hooks) > 0 {
		steps = append(steps, Step{
			Title: "shell-rc",
			Kind:  KindLink,
			Hooks: hooks,
		})
	}
	return steps, nil
}

func hasPackage(plans []Plan) bool {
	for _, p := range plans {
		if p.Method == pm.MethodPackage {
			return true
		}
	}
	return false
}

func itemStep(p Plan, euid int) (Step, error) {
	st := Step{
		ItemID:      p.ID,
		Manager:     p.Manager,
		Method:      p.Method,
		ProbeFiles:  append([]string(nil), p.ProbeFiles...),
		ReleaseRepo: p.ReleaseRepo,
	}
	if len(p.ProbeGroups) > 0 {
		st.ProbeGroups = make([][]string, len(p.ProbeGroups))
		for i, g := range p.ProbeGroups {
			st.ProbeGroups[i] = append([]string(nil), g...)
		}
	}
	switch p.Method {
	case pm.MethodPackage:
		argv, err := pm.BuildInstall(p.Manager, p.Packages)
		if err != nil {
			return Step{}, err
		}
		argv = pm.WithPrivilege(argv, pm.NeedsPrivilege(p.Manager), euid)
		if err := pm.ValidateArgv(argv); err != nil {
			return Step{}, err
		}
		st.Kind = KindPackage
		st.Title = "install " + string(p.ID)
		st.Argv = argv
	case pm.MethodRelease:
		if p.ReleaseRepo == "" {
			return Step{}, fmt.Errorf("%s 缺少 release repo", p.ID)
		}
		st.Kind = KindRelease
		st.Title = "release " + string(p.ID)
	case pm.MethodPreinstalled:
		st.Kind = KindProbe
		st.Title = "probe " + string(p.ID)
	case pm.MethodXcodeCLT:
		st.Kind = KindXcode
		st.Title = "xcode " + string(p.ID)
	case pm.MethodGoTarball:
		st.Kind = KindGo
		st.Title = "go-tarball " + string(p.ID)
	case pm.MethodRustup:
		st.Kind = KindRustup
		st.Title = "rustup " + string(p.ID)
	default:
		return Step{}, fmt.Errorf("%s 未知的安裝方法 %s", p.ID, p.Method)
	}
	if p.Fallback != nil {
		fb, err := itemStep(Plan{Plan: *p.Fallback, Manager: p.Manager}, euid)
		if err != nil {
			return Step{}, err
		}
		st.Fallback = &fb
	}
	return st, nil
}

func plannedHooks(plans []Plan) []string {
	var local, star, fzf, cargo, gopath bool
	for _, p := range plans {
		if p.NeedsLocalBin {
			local = true
		}
		switch p.ID {
		case "starship":
			star = true
		case "fzf":
			fzf = true
		case "rust":
			if p.Method == pm.MethodRustup {
				cargo = true
			}
		case "go":
			if p.Method == pm.MethodGoTarball {
				gopath = true
			}
		}
	}
	var hooks []string
	if local {
		hooks = append(hooks, "local-bin")
	}
	if star {
		hooks = append(hooks, "starship")
	}
	if fzf {
		hooks = append(hooks, "fzf")
	}
	if cargo {
		hooks = append(hooks, "rust")
	}
	if gopath {
		hooks = append(hooks, "go")
	}
	return hooks
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
