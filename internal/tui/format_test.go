package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/platform"
)

func TestFormatItem(t *testing.T) {
	git := catalog.Item{ID: "git", Name: "git", Default: true}
	zsh := catalog.Item{ID: "zsh", Name: "zsh", Default: false}
	nvim := catalog.Item{ID: "neovim", Name: "neovim", Default: false}
	got := []string{
		FormatItem(git, true),
		FormatItem(git, false),
		FormatItem(zsh, false),
		FormatItem(nvim, true),
	}
	want := []string{
		"* [x] git",
		"* [ ] git",
		"  [ ] zsh",
		"  [x] neovim",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d %q", i, got[i])
		}
	}
}

func TestFormatHeader(t *testing.T) {
	cases := []struct {
		info platform.Info
		want string
	}{
		{
			info: platform.Info{
				OS:      platform.OSInfo{SystemText: "Fedora Linux 45 (WSL) [GOOS=linux]"},
				Arch:    platform.ArchInfo{Family: "x86_64", GOARCH: "amd64", UnameM: "x86_64"},
				Manager: "dnf",
			},
			want: "系統: Fedora Linux 45 (WSL) [GOOS=linux]    架構: x86_64（GOARCH=amd64, uname-m=x86_64）    套件: dnf",
		},
		{
			info: platform.Info{
				OS:      platform.OSInfo{SystemText: "macOS [GOOS=darwin]"},
				Arch:    platform.ArchInfo{Family: "ARM", GOARCH: "arm64", UnameM: "aarch64"},
				Manager: "brew",
			},
			want: "系統: macOS [GOOS=darwin]    架構: ARM（GOARCH=arm64, uname-m=aarch64）    套件: brew",
		},
		{
			info: platform.Info{
				OS:      platform.OSInfo{SystemText: "Ubuntu 24.04.1 LTS [GOOS=linux, WSL]"},
				Arch:    platform.ArchInfo{Family: "i386", GOARCH: "386", UnameM: "i686"},
				Manager: "apt",
			},
			want: "系統: Ubuntu 24.04.1 LTS [GOOS=linux, WSL]    架構: i386（GOARCH=386, uname-m=i686）    套件: apt",
		},
	}
	for _, tc := range cases {
		got := FormatHeader(tc.info)
		if got != tc.want {
			t.Fatalf("\n%s\n%s", got, tc.want)
		}
		if strings.Contains(got, "RAM") || strings.Contains(got, "i86)") {
			t.Fatalf("bad family text %s", got)
		}
	}
	unknown := FormatHeader(platform.Info{OS: platform.OSInfo{SystemText: "linux [GOOS=linux]"}, Arch: platform.ArchInfo{Family: "x86_64", GOARCH: "amd64"}})
	if !strings.Contains(unknown, "uname-m=未知") || !strings.Contains(unknown, "套件: 未知") {
		t.Fatalf("%s", unknown)
	}
}

func TestPlaceHeaderCentersAndTruncates(t *testing.T) {
	text := FormatHeader(fedoraInfo())
	if ansi.StringWidth(text) != 101 {
		t.Fatalf("header width %d", ansi.StringWidth(text))
	}
	vis, left := placeHeader(text, 101)
	if left != 0 || vis != text {
		t.Fatalf("101 left %d %q", left, vis)
	}
	vis, left = placeHeader(text, 120)
	if left != 9 || vis != text {
		t.Fatalf("120 left %d", left)
	}
	const want80 = "系統: Fedora Linux 45 (WSL) [GOOS=linux]    架構: x86_64（GOARCH=amd64, uname-m…"
	vis, left = placeHeader(text, 80)
	if left != 0 || vis != want80 || ansi.StringWidth(vis) != 80 {
		t.Fatalf("80 left %d width %d %q", left, ansi.StringWidth(vis), vis)
	}
	vis, left = placeHeader(text, 4)
	if left != 0 || vis != "系…" || ansi.StringWidth(vis) != 3 {
		t.Fatalf("4 left %d width %d %q", left, ansi.StringWidth(vis), vis)
	}
	vis, left = placeHeader(text, 2)
	if left != 0 || vis != "…" || ansi.StringWidth(vis) != 1 {
		t.Fatalf("2 left %d width %d %q", left, ansi.StringWidth(vis), vis)
	}

	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	lines := frameLines(render(m, false))
	if lines[0] != want80 || strings.Contains(lines[0], "套件: dnf") {
		t.Fatalf("plain 80 %q", lines[0])
	}
	if lines[1] != "  基礎" {
		t.Fatalf("next %q", lines[1])
	}
	if strings.Contains(lines[0], "\x1b") {
		t.Fatal("plain header ansi")
	}

	m.width = 4
	lines = frameLines(render(m, false))
	if lines[0] != "系… " {
		t.Fatalf("plain 4 %q", lines[0])
	}
	m.width = 120
	lines = frameLines(render(m, false))
	if !strings.Contains(lines[0], text) || ansi.StringWidth(lines[0]) != 120 {
		t.Fatalf("plain 120 %q", lines[0])
	}
	if !strings.HasPrefix(lines[0], strings.Repeat(" ", 9)) {
		t.Fatalf("plain 120 pad %q", lines[0])
	}
}

func TestPlainInstalledRows(t *testing.T) {
	if catalogNameWidth() != 10 {
		t.Fatalf("name width %d", catalogNameWidth())
	}
	m := New(fedoraInfo())
	m.width = 80
	m.height = 40
	m.installed = map[catalog.ID]bool{"whois": true, "zsh": true}
	m.force = true
	for i, id := range m.flat {
		if id == "whois" {
			m.cursor = i
		}
	}
	view := render(m, false)
	if strings.Contains(view, "\x1b") {
		t.Fatal("plain ansi")
	}
	whois := "  > * [x] whois" + strings.Repeat(" ", 55) + "(已安裝)  "
	zsh := "      [ ] zsh" + strings.Repeat(" ", 57) + "(已安裝)  "
	if !strings.Contains(view, whois) || !strings.Contains(view, zsh) {
		t.Fatalf("rows\n%s", view)
	}
	if ansi.StringWidth(whois) != 80 || ansi.StringWidth(zsh) != 80 {
		t.Fatalf("whois %d zsh %d", ansi.StringWidth(whois), ansi.StringWidth(zsh))
	}
	if !strings.Contains(view, "> * [x] whois") || !strings.Contains(view, "    [ ] zsh") {
		t.Fatal("old substrings")
	}
	git := lineContaining(view, "* [x] git")
	if strings.Contains(git, "(已安裝)") || ansi.StringWidth(git) == 80 {
		t.Fatal("git marked or padded")
	}

	m.width = 100
	m.cursor = 0
	m.installed = map[catalog.ID]bool{"wget": true}
	view = render(m, false)
	wget := "    * [x] wget" + strings.Repeat(" ", 8) + "Fedora 套件是 wget2-wget" + strings.Repeat(" ", 44) + "(已安裝)  "
	if !strings.Contains(view, wget) || ansi.StringWidth(wget) != 100 {
		t.Fatalf("wget %d\n%s", ansi.StringWidth(wget), view)
	}

	m.width = 4
	m.installed = map[catalog.ID]bool{"git": true}
	four := lineContaining(render(m, false), "(已")
	if four != "(已…" {
		t.Fatalf("width 4 %q", four)
	}

	m.width = 80
	m.phase = phaseInstalling
	m.steps = []install.Step{{Title: "install whois"}}
	m.stepState = []string{"satisfied"}
	m.curTitle = "install whois"
	inst := render(m, false)
	if strings.Contains(inst, "(已安裝)") || !strings.Contains(inst, "已安裝  install whois") {
		t.Fatal(inst)
	}
	m.phase = phaseReport
	m.results = []install.ItemResult{{ID: "whois", Status: "satisfied"}}
	rep := render(m, false)
	if strings.Contains(rep, "(已安裝)") || !strings.Contains(rep, "whois  satisfied") {
		t.Fatal(rep)
	}
	m.phase = phaseFailed
	m.width = 80
	m.installed = map[catalog.ID]bool{"whois": true}
	failed := render(m, false)
	if !strings.Contains(failed, "(已安裝)") || !strings.Contains(failed, "enter") || !strings.Contains(failed, "d 只顯示計畫") {
		t.Fatal(failed)
	}
	if m.scroll != 0 {
		t.Fatal("scroll written")
	}
}
