package release_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/release"
)

func TestMatchStarshipAsset(t *testing.T) {
	names := []string{
		"starship-x86_64-unknown-linux-gnu.tar.gz",
		"starship-x86_64-unknown-linux-gnu.tar.gz.sha256",
		"starship-x86_64-unknown-linux-musl.tar.gz",
		"starship-aarch64-apple-darwin.tar.gz",
		"starship-x86_64-apple-darwin.tar.gz",
		"extra-starship-x86_64-unknown-linux-gnu.tar.gz",
	}
	got, err := release.MatchStarship(names[:5], release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != "starship-x86_64-unknown-linux-gnu.tar.gz" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = release.MatchStarship(names[:5], release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "musl"})
	if err != nil || got != "starship-x86_64-unknown-linux-musl.tar.gz" {
		t.Fatalf("musl %s %v", got, err)
	}
	got, err = release.MatchStarship(names[:5], release.AssetQuery{GOOS: "darwin", GOARCH: "amd64"})
	if err != nil || got != "starship-x86_64-apple-darwin.tar.gz" {
		t.Fatalf("darwin %s %v", got, err)
	}
	_, err = release.MatchStarship(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err == nil || !strings.Contains(err.Error(), "多個資產符合") {
		t.Fatalf("dup %v", err)
	}
	_, err = release.MatchStarship(names, release.AssetQuery{GOOS: "linux", GOARCH: "ppc64le"})
	want := "不支援的 GOARCH=ppc64le：此安裝方法沒有對應的上游資產或 triple"
	if err == nil || err.Error() != want {
		t.Fatalf("arch %v", err)
	}
}

func TestMatchEzaAsset(t *testing.T) {
	names := []string{
		"eza_x86_64-unknown-linux-gnu.tar.gz",
		"eza-aarch64-unknown-linux-gnu.tar.gz",
		"not-eza_x86_64-unknown-linux-gnu.tar.gz",
	}
	got, err := release.MatchEza(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != "eza_x86_64-unknown-linux-gnu.tar.gz" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = release.MatchEza(names, release.AssetQuery{GOOS: "linux", GOARCH: "arm64", Libc: "gnu"})
	if err != nil || got != "eza-aarch64-unknown-linux-gnu.tar.gz" {
		t.Fatalf("arm64 %s %v", got, err)
	}
}

func TestMatchZellijAsset(t *testing.T) {
	both := []string{
		"zellij-x86_64-unknown-linux-gnu.tar.gz",
		"zellij-x86_64-unknown-linux-musl.tar.gz",
	}
	got, err := release.MatchZellij(both, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != both[0] {
		t.Fatalf("gnu %s %v", got, err)
	}
	got, err = release.MatchZellij(both, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "musl"})
	if err != nil || got != both[1] {
		t.Fatalf("musl %s %v", got, err)
	}
	onlyMusl := []string{both[1]}
	got, err = release.MatchZellij(onlyMusl, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != both[1] {
		t.Fatalf("fallback %s %v", got, err)
	}
}

func TestMatchFastfetchAsset(t *testing.T) {
	names := []string{
		"fastfetch-linux-amd64.tar.gz",
		"fastfetch-linux-aarch64.tar.gz",
		"fastfetch-linux-armv7l.tar.gz",
		"fastfetch-macos-universal.tar.gz",
		"fastfetch.deb",
	}
	got, err := release.MatchFastfetch(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"})
	if err != nil || got != "fastfetch-linux-amd64.tar.gz" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = release.MatchFastfetch(names, release.AssetQuery{GOOS: "linux", GOARCH: "arm", UnameM: "armv7l"})
	if err != nil || got != "fastfetch-linux-armv7l.tar.gz" {
		t.Fatalf("arm %s %v", got, err)
	}
	got, err = release.MatchFastfetch(names, release.AssetQuery{GOOS: "darwin", GOARCH: "arm64"})
	if err != nil || got != "fastfetch-macos-universal.tar.gz" {
		t.Fatalf("mac %s %v", got, err)
	}
	_, err = release.MatchFastfetch(names, release.AssetQuery{GOOS: "linux", GOARCH: "arm", UnameM: "armv5l"})
	if err == nil || err.Error() != "不支援的 GOARCH=arm：此安裝方法沒有對應的上游資產或 triple" {
		t.Fatalf("bad arm %v", err)
	}
	_, err = release.MatchFastfetch([]string{"other.tar.gz"}, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"})
	if err == nil || !strings.Contains(err.Error(), "other.tar.gz") {
		t.Fatalf("missing %v", err)
	}
}

func TestMatchRipgrepAsset(t *testing.T) {
	names := []string{
		"ripgrep-15.1.0-x86_64-unknown-linux-musl.tar.gz",
		"ripgrep-15.1.0-x86_64-unknown-linux-gnu.tar.gz",
	}
	got, err := release.MatchRipgrep(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != names[0] {
		t.Fatalf("%s %v", got, err)
	}
}

func TestMatchFdAsset(t *testing.T) {
	names := []string{
		"fd-v10.2.0-x86_64-unknown-linux-gnu.tar.gz",
		"fd-v10.2.0-x86_64-unknown-linux-musl.tar.gz",
	}
	got, err := release.MatchFd(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Libc: "gnu"})
	if err != nil || got != names[0] {
		t.Fatalf("%s %v", got, err)
	}
}

func TestMatchBatAsset(t *testing.T) {
	names := []string{
		"bat-v0.24.0-x86_64-unknown-linux-gnu.tar.gz",
		"bat-v0.24.0-aarch64-unknown-linux-musl.tar.gz",
	}
	got, err := release.MatchBat(names, release.AssetQuery{GOOS: "linux", GOARCH: "arm64", Libc: "musl"})
	if err != nil || got != names[1] {
		t.Fatalf("%s %v", got, err)
	}
}

func TestMatchFzfAsset(t *testing.T) {
	names := []string{
		"fzf-0.75.0-linux_amd64.tar.gz",
		"fzf-0.75.0-darwin_amd64.tar.gz",
		"fzf-0.75.0-linux_arm64.tar.gz",
	}
	got, err := release.MatchFzf(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"})
	if err != nil || got != "fzf-0.75.0-linux_amd64.tar.gz" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = release.MatchFzf(names, release.AssetQuery{GOOS: "darwin", GOARCH: "amd64"})
	if err != nil || got != "fzf-0.75.0-darwin_amd64.tar.gz" {
		t.Fatalf("darwin %s %v", got, err)
	}
	_, err = release.MatchFzf(names, release.AssetQuery{GOOS: "linux", GOARCH: "arm", UnameM: "arm"})
	want := "不支援的 GOARCH=arm：uname-m 不足以選擇 linux_armv5、linux_armv6 或 linux_armv7"
	if err == nil || err.Error() != want {
		t.Fatalf("arm %v", err)
	}
	_, err = release.MatchFzf(names, release.AssetQuery{GOOS: "linux", GOARCH: "386"})
	if err == nil || err.Error() != "不支援的 GOARCH=386：此安裝方法沒有對應的上游資產或 triple" {
		t.Fatalf("386 %v", err)
	}
}

func TestMatchNeovimAsset(t *testing.T) {
	names := []string{"nvim-linux-x86_64.tar.gz", "nvim-linux-arm64.tar.gz", "nvim-macos-arm64.tar.gz", "nvim-linux-x86_64.appimage"}
	got, err := release.MatchNeovim(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"})
	if err != nil || got != "nvim-linux-x86_64.tar.gz" {
		t.Fatalf("%s %v", got, err)
	}
	_, err = release.MatchNeovim(names, release.AssetQuery{GOOS: "linux", GOARCH: "ppc64le"})
	if err == nil || !strings.HasPrefix(err.Error(), "不支援的 GOARCH=ppc64le：") {
		t.Fatalf("%v", err)
	}
}

func TestMatchMitosAsset(t *testing.T) {
	names := []string{
		"mitos-v0.2.1-x86_64-linux.tar.xz",
		"mitos-v0.2.1-aarch64-macos.tar.xz",
		"mitos_0.2.1_amd64.deb",
	}
	got, err := release.MatchMitos(names, release.AssetQuery{GOOS: "linux", GOARCH: "amd64", Version: "0.2.1"})
	if err != nil || got != names[0] {
		t.Fatalf("%s %v", got, err)
	}
	got, err = release.MatchMitos(names, release.AssetQuery{GOOS: "darwin", GOARCH: "arm64", Version: "0.2.1"})
	if err != nil || got != names[1] {
		t.Fatalf("mac %s %v", got, err)
	}
	_, err = release.MatchMitos(names, release.AssetQuery{GOOS: "linux", GOARCH: "386", Version: "0.2.1"})
	if err == nil || err.Error() != "不支援的 GOARCH=386：此安裝方法沒有對應的上游資產或 triple" {
		t.Fatalf("%v", err)
	}
}

func TestChecksumRequired(t *testing.T) {
	_, err := release.ExpectedSHA256("tool.tar.gz", map[string][]byte{"tool.tar.gz": []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "沒有 checksum") {
		t.Fatalf("%v", err)
	}
}

func TestChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.tar.gz")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := release.VerifyFile(path, strings.Repeat("ab", 32))
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("%v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("mismatch should delete the temp file")
	}
}

func TestSidecarSha256File(t *testing.T) {
	body := []byte("hello")
	sum := sha256.Sum256(body)
	hexsum := hex.EncodeToString(sum[:])
	got, err := release.ExpectedSHA256("foo.tar.gz", map[string][]byte{
		"foo.tar.gz.sha256": []byte(hexsum + "\n"),
	})
	if err != nil || got != hexsum {
		t.Fatalf("sidecar %s %v", got, err)
	}
	got, err = release.ExpectedSHA256("foo.tar.gz", map[string][]byte{
		"SHA256SUMS": []byte(hexsum + "  foo.tar.gz\n"),
	})
	if err != nil || got != hexsum {
		t.Fatalf("sums %s %v", got, err)
	}
	got, err = release.ExpectedSHA256("foo.tar.gz", map[string][]byte{
		"foo.tar.gz.sha256sum": []byte(hexsum + " *" + "foo.tar.gz\n"),
	})
	if err != nil || got != hexsum {
		t.Fatalf("sha256sum %s %v", got, err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.tar.gz")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := release.VerifyFile(path, got); err != nil {
		t.Fatal(err)
	}
}

func TestZellijStemChecksum(t *testing.T) {
	const asset = "zellij-x86_64-unknown-linux-musl.tar.gz"
	const stem = "zellij-x86_64-unknown-linux-musl.sha256sum"
	names := release.ChecksumNames(asset, []string{stem, "SHA256SUMS.txt", asset + ".sha256sum"})
	if len(names) < 2 || names[0] != asset+".sha256sum" || names[1] != stem {
		t.Fatalf("names %v", names)
	}
	body := []byte("zellij-bytes")
	sum := sha256.Sum256(body)
	hexsum := hex.EncodeToString(sum[:])
	files := map[string][]byte{
		stem: []byte(hexsum + "  target/x86_64-unknown-linux-musl/release/zellij\n"),
	}
	if _, err := release.ExpectedSHA256(asset, files); err == nil || !strings.Contains(err.Error(), "的列") {
		t.Fatalf("tarball lookup %v", err)
	}
	archive := tarGZ(t, []entry{{name: "zellij", body: string(body), mode: 0o755}})
	if err := release.VerifyStemBinary(archive, asset, files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]byte{stem: []byte(strings.Repeat("ab", 32) + "  zellij\n")}
	if err := release.VerifyStemBinary(archive, asset, bad); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("mismatch %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("mismatch should delete the archive")
	}
	missing := tarGZ(t, []entry{{name: "README", body: "nope"}})
	if err := release.VerifyStemBinary(missing, asset, files); err == nil {
		t.Fatal("missing inner file")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing inner file should delete the archive")
	}
	if err := release.VerifyStemBinary(missing, asset, map[string][]byte{}); err == nil || !strings.Contains(err.Error(), "的列") {
		t.Fatal("absent checksum file")
	}
}

func TestRejectPathTraversal(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "out")
	archive := tarGZ(t, []entry{{name: "../evil.txt", body: "x"}})
	if err := release.UnpackArchive(archive, dest); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.txt")); err == nil {
		t.Fatal("evil.txt escaped")
	}
	archive = tarGZ(t, []entry{{name: "/tmp/evil.txt", body: "x"}})
	if err := release.UnpackArchive(archive, dest); err == nil {
		t.Fatal("expected absolute path to fail")
	}
}

func TestRejectSymlinkEscape(t *testing.T) {
	dest := t.TempDir()
	archive := tarGZ(t, []entry{{name: "sub/link", link: "../../outside"}})
	if err := release.UnpackArchive(archive, dest); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
	if _, err := os.Lstat(filepath.Join(dest, "sub", "link")); err == nil {
		t.Fatal("symlink was created")
	}
}

func TestUnpackBinInTopDir(t *testing.T) {
	home := t.TempDir()
	archive := tarGZ(t, []entry{
		{name: "fd-v1/fd", body: "hello-fd", mode: 0o755},
		{name: "fd-v1/README.md", body: "nope"},
		{name: "fd-v1/autocomplete/fd.fish", body: "fish"},
	})
	spec := release.Spec{BinaryName: "fd", Layout: release.LayoutBinInTopDir}
	if err := release.InstallLayout(archive, home, spec); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, ".local", "bin", "fd"))
	if err != nil || string(got) != "hello-fd" {
		t.Fatalf("fd %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "README.md")); err == nil {
		t.Fatal("README installed")
	}
	info, err := os.Stat(filepath.Join(home, ".local", "bin", "fd"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", info)
	}
	binDir, err := os.Stat(filepath.Join(home, ".local", "bin"))
	if err != nil || binDir.Mode().Perm() != 0o755 {
		t.Fatalf("bin dir %v", binDir)
	}
}

func TestUnpackFastfetchTree(t *testing.T) {
	home := t.TempDir()
	top := "fastfetch-linux-amd64"
	archive := tarGZ(t, []entry{
		{name: top + "/usr/bin/fastfetch", body: "ff", mode: 0o755},
		{name: top + "/usr/bin/flashfetch", body: "flash", mode: 0o755},
		{name: top + "/usr/share/fastfetch/pci.ids", body: "ids"},
	})
	spec := release.Spec{BinaryName: "fastfetch", Layout: release.LayoutTree, ShareSubdir: "fastfetch", SymlinkRel: "usr/bin/fastfetch"}
	if err := release.InstallLayout(archive, home, spec); err != nil {
		t.Fatal(err)
	}
	share := filepath.Join(home, ".local", "share", "fastfetch", top, "usr", "share", "fastfetch", "pci.ids")
	if b, err := os.ReadFile(share); err != nil || string(b) != "ids" {
		t.Fatalf("share %q %v", b, err)
	}
	link := filepath.Join(home, ".local", "bin", "fastfetch")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "fastfetch", top, "usr", "bin", "fastfetch")
	if target != want {
		t.Fatalf("link %s want %s", target, want)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "flashfetch")); err == nil {
		t.Fatal("flashfetch was symlinked")
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "fastfetch", top, "usr", "bin", "flashfetch")); err != nil {
		t.Fatal("flashfetch binary was dropped from the tree")
	}
	// Reinstall over its own symlink (a second `tools apply`) succeeds.
	if err := release.InstallLayout(archive, home, spec); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	// A direct layout still refuses a symlink at ~/.local/bin/<name>.
	direct := tarGZ(t, []entry{{name: "fastfetch", body: "x", mode: 0o755}})
	if err := release.InstallLayout(direct, home, release.Spec{BinaryName: "fastfetch", Layout: release.LayoutBinDirect}); err == nil {
		t.Fatal("direct layout wrote through a symlink")
	}
}

func TestUnpackFastfetchTreeMissingShare(t *testing.T) {
	home := t.TempDir()
	top := "fastfetch-linux-amd64"
	archive := tarGZ(t, []entry{
		{name: top + "/usr/bin/fastfetch", body: "ff", mode: 0o755},
	})
	spec := release.Spec{BinaryName: "fastfetch", Layout: release.LayoutTree, ShareSubdir: "fastfetch", SymlinkRel: "usr/bin/fastfetch"}
	err := release.InstallLayout(archive, home, spec)
	if err == nil || !strings.Contains(err.Error(), "usr/share/fastfetch") {
		t.Fatalf("%v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(home, ".local", "bin", "fastfetch")); !os.IsNotExist(statErr) {
		t.Fatal("symlink created without usr/share/fastfetch")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local", "share", "fastfetch", top)); !os.IsNotExist(statErr) {
		t.Fatal("partial tree left behind")
	}
}

func TestUnpackNeovimTree(t *testing.T) {
	home := t.TempDir()
	top := "nvim-linux-x86_64"
	archive := tarGZ(t, []entry{
		{name: top + "/bin/nvim", body: "nvim-bin", mode: 0o755},
	})
	spec := release.Spec{BinaryName: "nvim", Layout: release.LayoutTree, ShareSubdir: "nvim", SymlinkRel: "bin/nvim"}
	if err := release.InstallLayout(archive, home, spec); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, ".local", "share", "nvim", top, "bin", "nvim")
	if b, err := os.ReadFile(bin); err != nil || string(b) != "nvim-bin" {
		t.Fatalf("nvim %q %v", b, err)
	}
	target, err := os.Readlink(filepath.Join(home, ".local", "bin", "nvim"))
	if err != nil || target != bin {
		t.Fatalf("link %s %v", target, err)
	}
}

func TestGoArchAndRustTriple(t *testing.T) {
	body := []byte(`[{"version":"go1.27.3","stable":true,"files":[{"filename":"go1.27.3.linux-amd64.tar.gz","sha256":"abc"}]}]`)
	f, err := release.SelectGoFile(body, "linux", "amd64")
	if err != nil || f.Filename != "go1.27.3.linux-amd64.tar.gz" || f.URL != "https://go.dev/dl/go1.27.3.linux-amd64.tar.gz" {
		t.Fatalf("%+v %v", f, err)
	}
	_, err = release.SelectGoFile(body, "linux", "mipsle")
	if err == nil || err.Error() != "不支援的 GOARCH=mipsle：此安裝方法沒有對應的上游資產或 triple" {
		t.Fatalf("mips %v", err)
	}
	tri, err := release.RustTriple("linux", "arm", "armv7l", "musl")
	if err != nil || tri != "armv7-unknown-linux-musleabihf" {
		t.Fatalf("%s %v", tri, err)
	}
	_, err = release.RustTriple("linux", "arm", "armv5l", "gnu")
	if err == nil || err.Error() != "不支援的 GOARCH=arm：此安裝方法沒有對應的上游資產或 triple" {
		t.Fatalf("armv5 %v", err)
	}
	argv := release.RustupArgv("/tmp/rustup-init")
	want := []string{"/tmp/rustup-init", "-y", "--no-modify-path", "--default-toolchain", "stable"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("%v", argv)
	}
	if release.DetectLibc("alpine", nil) != "musl" {
		t.Fatal("alpine")
	}
	if release.DetectLibc("fedora", func(string) bool { return false }) != "gnu" {
		t.Fatal("fedora")
	}
	if release.DetectLibc("debian", func(p string) bool { return p == "/lib/ld-musl*" }) != "musl" {
		t.Fatal("loader")
	}
}

type entry struct {
	name string
	body string
	link string
	mode int64
}

func tarGZ(t *testing.T, entries []entry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{Name: e.name, Mode: mode}
		if e.link != "" {
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = e.link
			hdr.Size = 0
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.link == "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
