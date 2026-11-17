package install_test

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

// zig is split per platform: distro packages with the ziglang.org tarball as
// fallback, and the tarball alone on apt and yum systems.
func TestZigStepsPerPlatform(t *testing.T) {
	want := map[string]string{
		"apt":    "release:ziglang.org/zig",
		"yum":    "release:ziglang.org/zig",
		"dnf":    "sudo dnf install -y zig>ziglang.org/zig",
		"pacman": "sudo pacman -S --needed --noconfirm zig>ziglang.org/zig",
		"zypper": "sudo zypper --non-interactive install --no-recommends zig>ziglang.org/zig",
		"apk":    "sudo apk add zig>ziglang.org/zig",
		"xbps":   "sudo xbps-install -y -S zig>ziglang.org/zig",
		"brew":   "brew install zig>ziglang.org/zig",
	}
	for mgr, w := range want {
		plans, err := install.ResolveAll([]catalog.ID{"zig"}, mgr)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := install.BuildSteps(plans, 1000, "/home/u")
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		for _, s := range steps {
			if s.ItemID != "zig" {
				continue
			}
			switch s.Kind {
			case install.KindRelease:
				got = "release:" + s.ReleaseRepo
			case install.KindPackage:
				got = strings.Join(s.Argv, " ")
				if s.Fallback != nil {
					got += ">" + s.Fallback.ReleaseRepo
				}
			}
		}
		if got != w {
			t.Errorf("%s: %q, want %q", mgr, got, w)
		}
	}
}
