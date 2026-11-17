package release_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/release"
	"github.com/ulikunitz/xz"
	"golang.org/x/crypto/blake2b"
)

const zigIndex = `{
  "master": {"version": "0.18.0-dev.1+abc", "x86_64-linux": {"tarball": "https://ziglang.org/builds/zig-x86_64-linux-0.18.0-dev.1+abc.tar.xz", "shasum": "%[1]s"}},
  "0.9.1": {"x86_64-linux": {"tarball": "https://ziglang.org/download/0.9.1/zig-linux-x86_64-0.9.1.tar.xz", "shasum": "%[1]s"}},
  "0.17.1": {
    "date": "2026-10-01",
    "src": {"tarball": "https://ziglang.org/download/0.17.1/zig-0.17.1.tar.xz", "shasum": "%[1]s"},
    "x86_64-linux": {"tarball": "https://ziglang.org/download/0.17.1/zig-x86_64-linux-0.17.1.tar.xz", "shasum": "%[1]s"},
    "aarch64-linux": {"tarball": "https://ziglang.org/download/0.17.1/zig-aarch64-linux-0.17.1.tar.xz", "shasum": "%[1]s"},
    "x86-linux": {"tarball": "https://ziglang.org/download/0.17.1/zig-x86-linux-0.17.1.tar.xz", "shasum": "%[1]s"},
    "arm-linux": {"tarball": "https://ziglang.org/download/0.17.1/zig-arm-linux-0.17.1.tar.xz", "shasum": "%[1]s"},
    "aarch64-macos": {"tarball": "https://ziglang.org/download/0.17.1/zig-aarch64-macos-0.17.1.tar.xz", "shasum": "%[1]s"},
    "x86_64-macos": {"tarball": "https://ziglang.org/download/0.17.1/zig-x86_64-macos-0.17.1.tar.xz", "shasum": "%[1]s"}
  },
  "0.16.0": {"x86_64-linux": {"tarball": "https://ziglang.org/download/0.16.0/zig-x86_64-linux-0.16.0.tar.xz", "shasum": "%[1]s"}}
}`

func TestSelectZigFile(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	body := []byte(fmt.Sprintf(zigIndex, sum))
	cases := map[string]string{
		"linux/amd64":  "zig-x86_64-linux-0.17.1.tar.xz",
		"linux/arm64":  "zig-aarch64-linux-0.17.1.tar.xz",
		"linux/386":    "zig-x86-linux-0.17.1.tar.xz",
		"linux/arm":    "zig-arm-linux-0.17.1.tar.xz",
		"darwin/arm64": "zig-aarch64-macos-0.17.1.tar.xz",
		"darwin/amd64": "zig-x86_64-macos-0.17.1.tar.xz",
	}
	for k, want := range cases {
		goos, goarch, _ := strings.Cut(k, "/")
		f, err := release.SelectZigFile(body, goos, goarch)
		if err != nil || f.Filename != want || f.Version != "0.17.1" || f.SHA256 != sum || f.URL != "https://ziglang.org/download/0.17.1/"+want {
			t.Fatalf("%s: %+v %v", k, f, err)
		}
	}
	if _, err := release.SelectZigFile(body, "linux", "riscv64"); err == nil {
		t.Fatal("riscv64 has no asset in the newest stable")
	}
	if _, err := release.SelectZigFile(body, "windows", "amd64"); err == nil {
		t.Fatal("windows accepted")
	}
	if _, err := release.SelectZigFile([]byte(fmt.Sprintf(zigIndex, "")), "linux", "amd64"); err == nil {
		t.Fatal("missing sha256 accepted")
	}
	evil := strings.Replace(string(body), "https://ziglang.org/download/0.17.1/zig-x86_64", "https://evil.example/zig-x86_64", 1)
	if _, err := release.SelectZigFile([]byte(evil), "linux", "amd64"); err == nil {
		t.Fatal("foreign host accepted")
	}
}

type minisignKey struct {
	priv ed25519.PrivateKey
	id   [8]byte
	pub  string
}

func newMinisignKey(t *testing.T) minisignKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k := minisignKey{priv: priv}
	copy(k.id[:], "keyid123")
	raw := append(append([]byte("Ed"), k.id[:]...), pub...)
	k.pub = base64.StdEncoding.EncodeToString(raw)
	return k
}

func (k minisignKey) sign(data []byte, file string) []byte {
	h := blake2b.Sum512(data)
	sig := ed25519.Sign(k.priv, h[:])
	trusted := "timestamp:1793729090\tfile:" + file + "\thashed"
	global := ed25519.Sign(k.priv, append(append([]byte{}, sig...), trusted...))
	line := append(append([]byte("ED"), k.id[:]...), sig...)
	return []byte("untrusted comment: signature from minisign secret key\n" +
		base64.StdEncoding.EncodeToString(line) + "\ntrusted comment: " + trusted + "\n" +
		base64.StdEncoding.EncodeToString(global) + "\n")
}

func TestVerifyMinisign(t *testing.T) {
	k := newMinisignKey(t)
	data := []byte("zig tarball bytes")
	sig := k.sign(data, "zig.tar.xz")
	if err := release.VerifyMinisign(k.pub, sig, bytes.NewReader(data), "zig.tar.xz"); err != nil {
		t.Fatal(err)
	}
	if release.VerifyMinisign(k.pub, sig, bytes.NewReader([]byte("tampered")), "zig.tar.xz") == nil {
		t.Fatal("tampered data accepted")
	}
	if release.VerifyMinisign(k.pub, sig, bytes.NewReader(data), "other.tar.xz") == nil {
		t.Fatal("wrong file name accepted")
	}
	forged := bytes.Replace(sig, []byte("timestamp:1793729090"), []byte("timestamp:1793729091"), 1)
	if release.VerifyMinisign(k.pub, forged, bytes.NewReader(data), "zig.tar.xz") == nil {
		t.Fatal("edited trusted comment accepted")
	}
	other := newMinisignKey(t)
	if release.VerifyMinisign(other.pub, sig, bytes.NewReader(data), "zig.tar.xz") == nil {
		t.Fatal("other key accepted")
	}
	if release.VerifyMinisign(release.ZigPublicKey, sig, bytes.NewReader(data), "zig.tar.xz") == nil {
		t.Fatal("real zig key accepted a test signature")
	}
}

// The real ziglang.org 0.17.1 x86_64-linux signature must parse against the
// published key (key id matches); the data check is done by InstallZig.
func TestZigPublicKeyMatchesRealSignature(t *testing.T) {
	sig := []byte("untrusted comment: signature from minisign secret key\nRUSGOq2NVecA2R8ojCoURkRiXJxI84Hv4ISwRsDzdh61qdlHzveXv/ZMCk8rS0Gy4HOBoktvgvfbACTooKMQfpBFnWuTrhKkHgc=\ntrusted comment: timestamp:1793729090\tfile:zig-x86_64-linux-0.17.1.tar.xz\thashed\nY+QP52U3hd6UXp3NPwsgtO8I2VVRduHwIICDBVCgpZ4MVmYsQMrwsVnV8IVFoAcXGiJOTBSAForDAcziQHqxDg==\n")
	err := release.VerifyMinisign(release.ZigPublicKey, sig, bytes.NewReader([]byte("not the tarball")), "zig-x86_64-linux-0.17.1.tar.xz")
	if err == nil || !strings.Contains(err.Error(), "簽章驗證失敗") {
		t.Fatalf("want a data mismatch (key id accepted), got %v", err)
	}
}

type urlDoer map[string][]byte

func (u urlDoer) Do(req *http.Request) (*http.Response, error) {
	b, ok := u[req.URL.String()]
	if !ok {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func tarXZ(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(xw)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	xw.Close()
	return buf.Bytes()
}

func zigServer(t *testing.T, k minisignKey, tamper bool) (urlDoer, string) {
	name := "zig-x86_64-linux-0.17.1.tar.xz"
	url := "https://ziglang.org/download/0.17.1/" + name
	archive := tarXZ(t, map[string]string{
		"zig-x86_64-linux-0.17.1/zig":             "#!/bin/sh\necho 0.17.1\n",
		"zig-x86_64-linux-0.17.1/lib/std/std.zig": "pub const x = 1;\n",
	})
	sum := sha256.Sum256(archive)
	sig := k.sign(archive, name)
	if tamper {
		sig = k.sign([]byte("other"), name)
	}
	index := fmt.Sprintf(`{"0.17.1": {"x86_64-linux": {"tarball": %q, "shasum": %q}}}`, url, hex.EncodeToString(sum[:]))
	return urlDoer{release.ZigIndexURL: []byte(index), url: archive, url + ".minisig": sig}, name
}

func TestInstallZigTree(t *testing.T) {
	k := newMinisignKey(t)
	defer release.SetZigKeyForTest(k.pub)()
	srv, _ := zigServer(t, k, false)
	home := t.TempDir()
	c := &release.Client{Meta: srv, File: srv}
	q := release.AssetQuery{GOOS: "linux", GOARCH: "amd64"}
	if err := c.Install(context.Background(), release.ZigRepo, home, q); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".local", "bin", "zig")
	target, err := os.Readlink(link)
	want := filepath.Join(home, ".local", "share", "zig", "zig-x86_64-linux-0.17.1", "zig")
	if err != nil || target != want {
		t.Fatalf("link %q %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "zig", "zig-x86_64-linux-0.17.1", "lib", "std", "std.zig")); err != nil {
		t.Fatal("lib tree missing")
	}
}

func TestInstallZigRejectsBadSignature(t *testing.T) {
	k := newMinisignKey(t)
	defer release.SetZigKeyForTest(k.pub)()
	srv, _ := zigServer(t, k, true)
	home := t.TempDir()
	c := &release.Client{Meta: srv, File: srv}
	err := c.Install(context.Background(), release.ZigRepo, home, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"})
	if err == nil || !strings.Contains(err.Error(), "minisign") {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "zig")); err == nil {
		t.Fatal("installed despite a bad signature")
	}
	// A wrong sha256 in index.json fails before the signature is fetched.
	srv2, _ := zigServer(t, k, false)
	for u, b := range srv2 {
		if strings.HasSuffix(u, ".tar.xz") {
			srv2[u] = append(b, 0)
		}
	}
	c = &release.Client{Meta: srv2, File: srv2}
	if err := c.Install(context.Background(), release.ZigRepo, home, release.AssetQuery{GOOS: "linux", GOARCH: "amd64"}); err == nil {
		t.Fatal("sha256 mismatch accepted")
	}
}
