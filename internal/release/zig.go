package release

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// ZigRepo is the pseudo repo for the official ziglang.org tarball. It is not a
// GitHub repo: Client.Install sends it to InstallZig.
const ZigRepo = "ziglang.org/zig"

// ZigIndexURL lists every release with a sha256 per tarball.
const ZigIndexURL = "https://ziglang.org/download/index.json"

// ZigPublicKey is the minisign key published on https://ziglang.org/download/.
const ZigPublicKey = "RWSGOq2NVecA2UPNdBUZykf1CCb147pkmdtYxgb3Ti+JO/wCYvhbAb/U"

// zigPublicKey is what InstallZig trusts; tests swap it for a throwaway key.
var zigPublicKey = ZigPublicKey

type ZigFile struct {
	Version  string
	Filename string
	URL      string
	SHA256   string
}

// zigTargets maps GOOS/GOARCH to index.json keys, newest naming first.
func zigTargets(goos, goarch string) ([]string, error) {
	var osTok string
	switch goos {
	case "linux":
		osTok = "linux"
	case "darwin":
		osTok = "macos"
	default:
		return nil, unsupported(goarch)
	}
	var arches []string
	switch goarch {
	case "amd64":
		arches = []string{"x86_64"}
	case "arm64":
		arches = []string{"aarch64"}
	case "386":
		arches = []string{"x86", "i386"}
	case "arm":
		arches = []string{"arm", "armv7a"}
	case "riscv64":
		arches = []string{"riscv64"}
	case "ppc64le":
		arches = []string{"powerpc64le"}
	case "s390x":
		arches = []string{"s390x"}
	case "loong64":
		arches = []string{"loongarch64"}
	default:
		return nil, unsupported(goarch)
	}
	if osTok == "macos" && goarch != "amd64" && goarch != "arm64" {
		return nil, unsupported(goarch)
	}
	out := make([]string, 0, len(arches))
	for _, a := range arches {
		out = append(out, a+"-"+osTok)
	}
	return out, nil
}

// SelectZigFile picks the newest stable version (skipping "master" and
// pre-releases) that has a tarball for the platform.
func SelectZigFile(body []byte, goos, goarch string) (ZigFile, error) {
	targets, err := zigTargets(goos, goarch)
	if err != nil {
		return ZigFile{}, err
	}
	var index map[string]json.RawMessage
	if err := json.Unmarshal(body, &index); err != nil {
		return ZigFile{}, err
	}
	best, bestV := "", []int(nil)
	for k := range index {
		v, ok := stableVersion(k)
		if !ok {
			continue
		}
		if bestV == nil || versionLess(bestV, v) {
			best, bestV = k, v
		}
	}
	if best == "" {
		return ZigFile{}, fmt.Errorf("index.json 沒有 stable zig 版本")
	}
	var rel map[string]json.RawMessage
	if err := json.Unmarshal(index[best], &rel); err != nil {
		return ZigFile{}, err
	}
	for _, t := range targets {
		raw, ok := rel[t]
		if !ok {
			continue
		}
		var f struct {
			Tarball string `json:"tarball"`
			Shasum  string `json:"shasum"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return ZigFile{}, err
		}
		if !strings.HasPrefix(f.Tarball, "https://ziglang.org/") || !strings.HasSuffix(f.Tarball, ".tar.xz") {
			return ZigFile{}, fmt.Errorf("不接受的 zig 下載網址 %q", f.Tarball)
		}
		if !isHex64(strings.ToLower(f.Shasum)) {
			return ZigFile{}, fmt.Errorf("zig %s 沒有 sha256", t)
		}
		name := f.Tarball[strings.LastIndex(f.Tarball, "/")+1:]
		return ZigFile{Version: best, Filename: name, URL: f.Tarball, SHA256: strings.ToLower(f.Shasum)}, nil
	}
	return ZigFile{}, fmt.Errorf("zig %s 沒有 %s 的資產", best, strings.Join(targets, "/"))
}

func stableVersion(s string) ([]int, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, false
	}
	v := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		v[i] = n
	}
	return v, true
}

func versionLess(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// VerifyMinisign checks a minisign signature (prehashed "ED" or legacy "Ed")
// over data, including the trusted comment, and that the trusted comment
// names the expected file.
func VerifyMinisign(pubKey string, sig []byte, data io.Reader, filename string) error {
	pk, err := base64.StdEncoding.DecodeString(pubKey)
	if err != nil || len(pk) != 42 || string(pk[:2]) != "Ed" {
		return fmt.Errorf("minisign 公鑰格式錯誤")
	}
	lines := strings.Split(strings.ReplaceAll(string(sig), "\r\n", "\n"), "\n")
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "untrusted comment:") || !strings.HasPrefix(lines[2], "trusted comment: ") {
		return fmt.Errorf("minisign 簽章格式錯誤")
	}
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil || len(s) != 74 {
		return fmt.Errorf("minisign 簽章格式錯誤")
	}
	if !bytes.Equal(s[2:10], pk[2:10]) {
		return fmt.Errorf("minisign key id 不符")
	}
	pub := ed25519.PublicKey(pk[10:])
	var msg []byte
	switch string(s[:2]) {
	case "ED":
		h, _ := blake2b.New512(nil)
		if _, err := io.Copy(h, data); err != nil {
			return err
		}
		msg = h.Sum(nil)
	case "Ed":
		if msg, err = io.ReadAll(data); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支援的 minisign 演算法")
	}
	if !ed25519.Verify(pub, msg, s[10:]) {
		return fmt.Errorf("minisign 簽章驗證失敗")
	}
	trusted := strings.TrimPrefix(lines[2], "trusted comment: ")
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil || len(global) != 64 {
		return fmt.Errorf("minisign 全域簽章格式錯誤")
	}
	if !ed25519.Verify(pub, append(append([]byte{}, s[10:]...), trusted...), global) {
		return fmt.Errorf("minisign trusted comment 驗證失敗")
	}
	if filename != "" && !strings.Contains("\t"+trusted+"\t", "\tfile:"+filename+"\t") {
		return fmt.Errorf("minisign 簽的不是 %s", filename)
	}
	return nil
}

// InstallZig downloads the newest stable tarball from ziglang.org, checks
// the index.json sha256 and the minisign signature, then installs it as a
// tree under ~/.local/share/zig/<top> with ~/.local/bin/zig as a symlink.
func (c *Client) InstallZig(ctx context.Context, home string, q AssetQuery) error {
	if c.Meta == nil {
		return fmt.Errorf("沒有中繼資料 HTTP client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ZigIndexURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dotfiles-tools")
	resp, err := c.Meta.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ziglang.org HTTP %d", resp.StatusCode)
	}
	file, err := SelectZigFile(body, q.GOOS, q.GOARCH)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "dotfiles-tools-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, file.Filename)
	if err := c.Download(ctx, file.URL, archive); err != nil {
		return err
	}
	if err := VerifyFile(archive, file.SHA256); err != nil {
		return err
	}
	sigPath := archive + ".minisig"
	if err := c.Download(ctx, file.URL+".minisig", sigPath); err != nil {
		return fmt.Errorf("下載 minisign 簽章：%w", err)
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	err = VerifyMinisign(zigPublicKey, sig, f, file.Filename)
	f.Close()
	if err != nil {
		return err
	}
	spec, _ := SpecFor(ZigRepo)
	return InstallLayout(archive, home, spec)
}
