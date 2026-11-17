package platform_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/platform"
)

func TestParseOSRelease(t *testing.T) {
	in := `
# comment
ID=fedora
NAME="Fedora Linux"
VERSION="45 (Workstation Edition)"
ID_LIKE="rhel fedora"
PRETTY_NAME='Fedora Linux 45 (WSL)'
VERSION_ID=45

`
	osr, err := platform.ParseOSRelease(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if osr.ID != "fedora" || osr.Name != "Fedora Linux" || osr.VersionID != "45" {
		t.Fatalf("%+v", osr)
	}
	if osr.PrettyName != "Fedora Linux 45 (WSL)" {
		t.Fatalf("pretty %q", osr.PrettyName)
	}
	if osr.Version != "45 (Workstation Edition)" {
		t.Fatalf("version %q", osr.Version)
	}
	toks := strings.Fields(osr.IDLike)
	if len(toks) != 2 || toks[0] != "rhel" || toks[1] != "fedora" {
		t.Fatalf("ID_LIKE %q", osr.IDLike)
	}
}

func TestClassifyOS(t *testing.T) {
	cases := []struct {
		pretty, goos string
		wsl          bool
		empty        bool
		want         string
	}{
		{pretty: "Fedora Linux 45 (WSL)", goos: "linux", wsl: true, want: "Fedora Linux 45 (WSL) [GOOS=linux]"},
		{pretty: "Ubuntu 24.04.1 LTS", goos: "linux", wsl: true, want: "Ubuntu 24.04.1 LTS [GOOS=linux, WSL]"},
		{pretty: "Ubuntu 24.04.1 LTS", goos: "linux", wsl: false, want: "Ubuntu 24.04.1 LTS [GOOS=linux]"},
		{goos: "darwin", empty: true, want: "macOS [GOOS=darwin]"},
		{goos: "linux", empty: true, want: "linux [GOOS=linux]"},
	}
	for _, tc := range cases {
		var osr platform.OSRelease
		if !tc.empty {
			osr.PrettyName = tc.pretty
		}
		got := platform.ClassifyOS(tc.goos, osr, tc.wsl)
		if got.SystemText != tc.want {
			t.Fatalf("ClassifyOS(%q, wsl=%v) = %q, want %q", tc.goos, tc.wsl, got.SystemText, tc.want)
		}
	}
}
