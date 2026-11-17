package platform_test

import (
	"testing"

	"github.com/shuhi/dotfiles/internal/platform"
)

func TestClassifyArch(t *testing.T) {
	cases := []struct {
		goarch, uname, family string
	}{
		{"amd64", "x86_64", "x86_64"},
		{"386", "i386", "i386"},
		{"386", "i686", "i386"},
		{"386", "i86", "i386"},
		{"arm64", "aarch64", "ARM"},
		{"arm64", "arm64", "ARM"},
		{"arm", "arm", "ARM"},
		{"arm", "armv7l", "ARM"},
		{"arm", "armv6l", "ARM"},
		{"riscv64", "riscv64", "riscv64"},
		{"ppc64le", "ppc64le", "ppc64le"},
		{"s390x", "s390x", "s390x"},
		{"loong64", "loong64", "loong64"},
		{"loong64", "loongarch64", "loong64"},
		{"mips", "mips", "mips"},
		{"mipsle", "mipsle", "mipsle"},
		{"mips64", "mips64", "mips64"},
		{"mips64le", "mips64le", "mips64le"},
		{"", "", "unknown"},
		{"amd64", "", "x86_64"},
	}
	for _, tc := range cases {
		got := platform.ClassifyArch(tc.goarch, tc.uname)
		if got.Family != tc.family || got.GOARCH != tc.goarch || got.UnameM != tc.uname {
			t.Fatalf("ClassifyArch(%q,%q) = %+v, want family %s", tc.goarch, tc.uname, got, tc.family)
		}
		if got.Family == "RAM" || got.Family == "i86" {
			t.Fatalf("forbidden family %s", got.Family)
		}
	}
}
