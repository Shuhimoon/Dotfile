package install_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

func TestBuildStepsFedoraDefaults(t *testing.T) {
	plans, err := install.ResolveAll(catalog.DefaultIDs(), "dnf")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := install.BuildSteps(plans, 1000, "/home/user")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		title, id string
		argv      []string
	}
	sudo := func(pkg string) []string {
		return []string{"sudo", "dnf", "install", "-y", pkg}
	}
	want := []row{
		{"install git", "git", sudo("git")},
		{"install curl", "curl", sudo("curl")},
		{"install wget", "wget", sudo("wget2-wget")},
		{"install unzip", "unzip", sudo("unzip")},
		{"install rsync", "rsync", sudo("rsync")},
		{"install vim", "vim", sudo("vim-enhanced")},
		{"install less", "less", sudo("less")},
		{"install net-tools", "net-tools", sudo("net-tools")},
		{"install whois", "whois", sudo("whois")},
		{"install dnsutils", "dnsutils", sudo("bind-utils")},
		{"install traceroute", "traceroute", sudo("traceroute")},
		{"install eza", "eza", sudo("eza")},
		{"install trash", "trash", sudo("trash-cli")},
		{"install ripgrep", "ripgrep", sudo("ripgrep")},
		{"install fd", "fd", sudo("fd-find")},
		{"install bat", "bat", sudo("bat")},
		{"install jq", "jq", sudo("jq")},
		{"install fzf", "fzf", sudo("fzf")},
		{"release zellij", "zellij", nil},
		{"release mitos", "mitos", nil},
		{"config zellij", "zellij", nil},
		{"link-bash", "", nil},
	}
	if len(steps) != len(want) {
		t.Fatalf("len %d", len(steps))
	}
	for i, w := range want {
		st := steps[i]
		if st.Title != w.title || string(st.ItemID) != w.id {
			t.Fatalf("%d title/id %q %q", i, st.Title, st.ItemID)
		}
		if strings.Contains(st.Title, "無 sudo") {
			t.Fatalf("title %s", st.Title)
		}
		if w.argv == nil {
			if st.Argv != nil {
				t.Fatalf("%d argv %v", i, st.Argv)
			}
		} else if strings.Join(st.Argv, " ") != strings.Join(w.argv, " ") {
			t.Fatalf("%d argv %v", i, st.Argv)
		}
		if st.Title == "update" || strings.Contains(st.Title, "alias") {
			t.Fatalf("unexpected %s", st.Title)
		}
	}
	last := steps[len(steps)-1]
	if strings.Join(last.Hooks, ",") != "local-bin,fzf" {
		t.Fatalf("hooks %v", last.Hooks)
	}
}
