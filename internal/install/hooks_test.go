package install_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/linkbash"
	"github.com/shuhi/dotfiles/internal/pm"
)

func TestHooksFor(t *testing.T) {
	var fed []install.ItemResult
	for _, id := range catalog.DefaultIDs() {
		res := install.ItemResult{ID: id, Method: pm.MethodPackage, Status: "ok"}
		if id == "zellij" || id == "mitos" {
			res.Method = pm.MethodRelease
			res.NeedsLocalBin = true
		}
		fed = append(fed, res)
	}
	if got := joinHooks(install.HooksFor(fed)); got != "local-bin,fzf" {
		t.Fatalf("fedora %s", got)
	}
	only := []install.ItemResult{{ID: "fzf", Method: pm.MethodPackage, Status: "ok"}}
	if got := joinHooks(install.HooksFor(only)); got != "fzf" {
		t.Fatalf("fzf only %s", got)
	}
	// Order follows the fixed list, not the slice order.
	mixed := []install.ItemResult{
		{ID: "go", Method: pm.MethodGoTarball, Status: "ok"},
		{ID: "fzf", Method: pm.MethodPackage, Status: "ok"},
		{ID: "rust", Method: pm.MethodRustup, Status: "ok"},
		{ID: "eza", Method: pm.MethodRelease, Status: "ok", NeedsLocalBin: true},
	}
	if got := joinHooks(install.HooksFor(mixed)); got != "local-bin,fzf,cargo-env,go-path" {
		t.Fatalf("order %s", got)
	}
	failed := []install.ItemResult{
		{ID: "fzf", Method: pm.MethodPackage, Status: "failed", NeedsLocalBin: true},
		{ID: "rust", Method: pm.MethodRustup, Status: "pending-user"},
		{ID: "go", Method: pm.MethodGoTarball, Status: "failed"},
	}
	if got := install.HooksFor(failed); len(got) != 0 {
		t.Fatalf("failed %v", got)
	}
	// Package rust/go do not add cargo-env or go-path.
	pkg := []install.ItemResult{
		{ID: "rust", Method: pm.MethodPackage, Status: "ok"},
		{ID: "go", Method: pm.MethodPackage, Status: "ok"},
	}
	if got := install.HooksFor(pkg); len(got) != 0 {
		t.Fatalf("pkg %v", got)
	}
}

func joinHooks(ids []linkbash.HookID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ",")
}
