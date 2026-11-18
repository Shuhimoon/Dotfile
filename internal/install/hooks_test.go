package install_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

func TestBlocksFor(t *testing.T) {
	var fed []install.ItemResult
	for _, id := range catalog.DefaultIDs() {
		res := install.ItemResult{ID: id, Method: pm.MethodPackage, Status: "ok"}
		if id == "zellij" || id == "mitos" {
			res.Method = pm.MethodRelease
			res.NeedsLocalBin = true
		}
		fed = append(fed, res)
	}
	if got := joinHooks(install.BlocksFor(fed)); got != "local-bin,fzf,base" {
		t.Fatalf("fedora %s", got)
	}
	only := []install.ItemResult{{ID: "fzf", Method: pm.MethodPackage, Status: "ok"}}
	if got := joinHooks(install.BlocksFor(only)); got != "fzf,base" {
		t.Fatalf("fzf only %s", got)
	}
	// Order follows the fixed list, not the slice order.
	mixed := []install.ItemResult{
		{ID: "go", Method: pm.MethodGoTarball, Status: "ok"},
		{ID: "fzf", Method: pm.MethodPackage, Status: "ok"},
		{ID: "rust", Method: pm.MethodRustup, Status: "ok"},
		{ID: "eza", Method: pm.MethodRelease, Status: "ok", NeedsLocalBin: true},
	}
	if got := joinHooks(install.BlocksFor(mixed)); got != "local-bin,fzf,rust,go,base" {
		t.Fatalf("order %s", got)
	}
	failed := []install.ItemResult{
		{ID: "fzf", Method: pm.MethodPackage, Status: "failed", NeedsLocalBin: true},
		{ID: "rust", Method: pm.MethodRustup, Status: "pending-user"},
		{ID: "go", Method: pm.MethodGoTarball, Status: "failed"},
	}
	if got := joinHooks(install.BlocksFor(failed)); got != "base" {
		t.Fatalf("failed %v", got)
	}
	// Package rust/go do not add the rust or go blocks.
	pkg := []install.ItemResult{
		{ID: "rust", Method: pm.MethodPackage, Status: "ok"},
		{ID: "go", Method: pm.MethodPackage, Status: "ok"},
	}
	if got := joinHooks(install.BlocksFor(pkg)); got != "base" {
		t.Fatalf("pkg %v", got)
	}
}

func joinHooks(ids []shellrc.BlockID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ",")
}

func TestBlocksForStarshipAndSatisfied(t *testing.T) {
	cases := []struct {
		res  []install.ItemResult
		want string
	}{
		{[]install.ItemResult{{ID: "starship", Method: pm.MethodRelease, Status: "ok", NeedsLocalBin: true}}, "local-bin,starship,base"},
		{[]install.ItemResult{{ID: "starship", Method: pm.MethodPackage, Status: "satisfied"}}, "starship,base"},
		{[]install.ItemResult{{ID: "starship", Method: pm.MethodPackage, Status: "failed"}}, "base"},
		{[]install.ItemResult{{ID: "rust", Method: pm.MethodRustup, Status: "satisfied"}}, "base"},
	}
	for i, c := range cases {
		if got := joinHooks(install.BlocksFor(c.res)); got != c.want {
			t.Errorf("%d: %q want %q", i, got, c.want)
		}
	}
}
