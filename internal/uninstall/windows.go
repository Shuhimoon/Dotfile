package uninstall

import (
	"fmt"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

// RemovableOn is Removable for goos. Every Windows catalog item is removable
// (Windows has no locked basics category).
func RemovableOn(goos string, id catalog.ID) bool {
	if goos == "windows" {
		_, ok := pm.ScoopFor(id)
		return ok
	}
	return Removable(id)
}

// SupportedOn is Supported for goos and manager.
func SupportedOn(goos string, id catalog.ID, manager string) bool {
	if goos == "windows" {
		_, ok := pm.ScoopFor(id)
		return ok
	}
	return Supported(id, manager)
}

// WarningOn is NeedsWarning for goos. The python warning is about Linux
// package managers and does not apply to Scoop.
func WarningOn(goos string, id catalog.ID) (string, bool) {
	if goos == "windows" {
		return "", false
	}
	return NeedsWarning(id)
}

// BuildWindowsSteps is one "scoop uninstall <app>" per item in catalog order,
// plus the ssh-config step for git. No sudo on Windows.
func BuildWindowsSteps(ids []catalog.ID) ([]Step, error) {
	want := map[catalog.ID]bool{}
	for _, id := range ids {
		if _, ok := pm.ScoopFor(id); !ok {
			return nil, fmt.Errorf("Windows 不支援 %s", id)
		}
		want[id] = true
	}
	var steps []Step
	for _, id := range catalog.IDsFor("windows") {
		if !want[id] {
			continue
		}
		it, _ := pm.ScoopFor(id)
		argv := it.RemoveArgv()
		if err := pm.ValidateArgv(argv); err != nil {
			return nil, err
		}
		steps = append(steps, Step{ItemID: id, Title: string(id), Kind: KindPackage, Manager: pm.Scoop, Argv: argv})
		if id == "git" {
			steps = append(steps, Step{ItemID: id, Title: "ssh-config git", Kind: KindSSH})
		}
	}
	return steps, nil
}
