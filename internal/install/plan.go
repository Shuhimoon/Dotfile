package install

import (
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

// Plan is one resolved catalog row plus the manager that produced it.
type Plan struct {
	pm.Plan
	Manager string
}

type ItemResult struct {
	ID            catalog.ID
	Method        pm.Method
	Status        string // ok, satisfied, failed, pending-user, skipped, shadowed
	NeedsLocalBin bool
	Note          string
	ErrText       string
	Argv          []string
}

func ResolveAll(ids []catalog.ID, manager string) ([]Plan, error) {
	seen := map[catalog.ID]bool{}
	var out []Plan
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		p, err := pm.Resolve(id, manager)
		if err != nil {
			return nil, err
		}
		out = append(out, Plan{Plan: p, Manager: manager})
	}
	return out, nil
}
