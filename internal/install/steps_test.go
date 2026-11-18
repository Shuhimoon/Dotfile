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
		{"shell-rc", "", nil},
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
	if strings.Join(last.Hooks, ",") != "local-bin,fzf,base" {
		t.Fatalf("hooks %v", last.Hooks)
	}
}

// --config-only builds config steps (configcopy.ToolIDs order) plus
// zsh-plugins when zsh is selected, and nothing else.
func TestBuildConfigSteps(t *testing.T) {
	cases := []struct {
		ids  []catalog.ID
		want []string
	}{
		{[]catalog.ID{"zellij", "starship", "zsh"}, []string{"config:config zsh", "config:config starship", "config:config zellij", "zsh-plugins:zsh-plugins"}},
		{[]catalog.ID{"zellij", "starship"}, []string{"config:config starship", "config:config zellij"}},
		{[]catalog.ID{"zsh"}, []string{"config:config zsh", "zsh-plugins:zsh-plugins"}},
	}
	for _, c := range cases {
		steps, err := install.BuildConfigSteps(c.ids, nil, 1000)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, st := range steps {
			if len(st.Argv) != 0 {
				t.Fatalf("%v: argv %v", c.ids, st.Argv)
			}
			got = append(got, string(st.Kind)+":"+st.Title)
		}
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%v:\n%s", c.ids, strings.Join(got, "\n"))
		}
	}
	if _, err := install.BuildConfigSteps([]catalog.ID{"git"}, nil, 1000); err == nil {
		t.Fatal("git has no config")
	}
}

// With git missing, the git steps BuildSteps makes (update when the manager
// has one, then install git; no shell-rc) go right before zsh-plugins, and
// only when zsh is selected.
func TestBuildConfigStepsInstallsMissingGit(t *testing.T) {
	for _, c := range []struct {
		manager string
		euid    int
		want    []string
	}{
		{"dnf", 1000, []string{"config:config zsh", "config:config zellij", "package:install git:sudo dnf install -y git", "zsh-plugins:zsh-plugins"}},
		{"dnf", 0, []string{"config:config zsh", "config:config zellij", "package:install git:dnf install -y git", "zsh-plugins:zsh-plugins"}},
		{"apt", 1000, []string{"config:config zsh", "config:config zellij", "update:update:sudo apt-get update", "package:install git:sudo apt-get install -y --no-install-recommends git", "zsh-plugins:zsh-plugins"}},
	} {
		git, err := install.ResolveAll([]catalog.ID{"git"}, c.manager)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildConfigSteps([]catalog.ID{"zellij", "zsh"}, git, c.euid)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, st := range steps {
			s := string(st.Kind) + ":" + st.Title
			if len(st.Argv) > 0 {
				s += ":" + strings.Join(st.Argv, " ")
			}
			got = append(got, s)
		}
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s euid %d:\n%s", c.manager, c.euid, strings.Join(got, "\n"))
		}
		noZsh, _ := install.BuildConfigSteps([]catalog.ID{"zellij"}, git, c.euid)
		if len(noZsh) != 1 {
			t.Errorf("git steps without zsh: %+v", noZsh)
		}
	}
}
