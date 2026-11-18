package release_test

import (
	"testing"

	"github.com/shuhi/dotfiles/internal/release"
)

// Real asset list of zellij v0.46.0 (the -no-web builds made musl and macOS
// ambiguous before).
var zellij046 = []string{
	"zellij-aarch64-apple-darwin.sha256sum", "zellij-aarch64-apple-darwin.tar.gz",
	"zellij-aarch64-unknown-linux-gnu.sha256sum", "zellij-aarch64-unknown-linux-gnu.tar.gz",
	"zellij-aarch64-unknown-linux-musl.sha256sum", "zellij-aarch64-unknown-linux-musl.tar.gz",
	"zellij-desktop-files.sha256sum", "zellij-desktop-files.tar.gz",
	"zellij-no-web-aarch64-apple-darwin.sha256sum", "zellij-no-web-aarch64-apple-darwin.tar.gz",
	"zellij-no-web-aarch64-unknown-linux-musl.sha256sum", "zellij-no-web-aarch64-unknown-linux-musl.tar.gz",
	"zellij-no-web-x86_64-apple-darwin.sha256sum", "zellij-no-web-x86_64-apple-darwin.tar.gz",
	"zellij-no-web-x86_64-pc-windows-msvc-installer.msi", "zellij-no-web-x86_64-pc-windows-msvc.zip",
	"zellij-no-web-x86_64-unknown-linux-musl.sha256sum", "zellij-no-web-x86_64-unknown-linux-musl.tar.gz",
	"zellij-x86_64-apple-darwin.sha256sum", "zellij-x86_64-apple-darwin.tar.gz",
	"zellij-x86_64-pc-windows-msvc-installer.msi", "zellij-x86_64-pc-windows-msvc.zip",
	"zellij-x86_64-unknown-linux-gnu.sha256sum", "zellij-x86_64-unknown-linux-gnu.tar.gz",
	"zellij-x86_64-unknown-linux-musl.sha256sum", "zellij-x86_64-unknown-linux-musl.tar.gz",
}

func TestMatchZellij046RealAssets(t *testing.T) {
	cases := []struct {
		q    release.AssetQuery
		want string
	}{
		{release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"}, "zellij-x86_64-unknown-linux-gnu.tar.gz"},
		{release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "musl"}, "zellij-x86_64-unknown-linux-musl.tar.gz"},
		{release.AssetQuery{GOOS: "linux", GOARCH: "arm64", Libc: "gnu"}, "zellij-aarch64-unknown-linux-gnu.tar.gz"},
		{release.AssetQuery{GOOS: "linux", GOARCH: "arm64", Libc: "musl"}, "zellij-aarch64-unknown-linux-musl.tar.gz"},
		{release.AssetQuery{GOOS: "darwin", GOARCH: "arm64"}, "zellij-aarch64-apple-darwin.tar.gz"},
		{release.AssetQuery{GOOS: "darwin", GOARCH: "amd64"}, "zellij-x86_64-apple-darwin.tar.gz"},
	}
	for _, c := range cases {
		got, err := release.MatchZellij(zellij046, c.q)
		if err != nil || got != c.want {
			t.Fatalf("%+v: got %q %v want %q", c.q, got, err, c.want)
		}
	}
	// Only a no-web build left: still usable.
	got, err := release.MatchZellij([]string{"zellij-no-web-x86_64-unknown-linux-musl.tar.gz"}, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "musl"})
	if err != nil || got != "zellij-no-web-x86_64-unknown-linux-musl.tar.gz" {
		t.Fatalf("no-web only %q %v", got, err)
	}
}
