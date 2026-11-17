package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Preference: <asset>.sha256, <asset>.sha256sum (tarball digest),
// <stem>.sha256sum where stem is the asset without .tar.gz (digest of the
// extracted file named by that line), SHA256SUMS, then *checksums*.txt.
var checksumRe = regexp.MustCompile(`(?i)(checksums|sha256sums).*\.txt$`)

func ChecksumNames(asset string, names []string) []string {
	var side, sideSum, stemSum, sum, re []string
	stem, stemOK := stemSumName(asset)
	for _, n := range names {
		switch {
		case n == asset+".sha256":
			side = append(side, n)
		case n == asset+".sha256sum":
			sideSum = append(sideSum, n)
		case stemOK && n == stem:
			stemSum = append(stemSum, n)
		case strings.EqualFold(n, "SHA256SUMS"):
			sum = append(sum, n)
		case checksumRe.MatchString(n):
			re = append(re, n)
		}
	}
	out := append(append(side, sideSum...), stemSum...)
	out = append(out, sum...)
	return append(out, re...)
}

func stemSumName(asset string) (string, bool) {
	const suf = ".tar.gz"
	if !strings.HasSuffix(asset, suf) {
		return "", false
	}
	name := strings.TrimSuffix(asset, suf) + ".sha256sum"
	if name == asset+".sha256sum" {
		return "", false
	}
	return name, true
}

func ExpectedSHA256(asset string, files map[string][]byte) (string, error) {
	var names []string
	for n := range files {
		names = append(names, n)
	}
	cands := ChecksumNames(asset, names)
	if len(cands) == 0 {
		return "", fmt.Errorf("沒有 checksum：%s", asset)
	}
	for _, name := range cands {
		if hexsum, ok := hashFor(asset, name, files[name]); ok {
			return hexsum, nil
		}
	}
	return "", fmt.Errorf("checksum 沒有 %s 的列", asset)
}

// VerifyStemBinary checks the extracted regular file named by <stem>.sha256sum.
// The digest is not a tarball hash. A missing file, missing line, or mismatch fails.
func VerifyStemBinary(archivePath, asset string, files map[string][]byte) error {
	base, hexsum, ok := stemBinary(asset, files)
	if !ok {
		return fmt.Errorf("checksum 沒有 %s 的列", asset)
	}
	if err := verifyExtracted(archivePath, base, hexsum); err != nil {
		os.Remove(archivePath)
		return err
	}
	return nil
}

func stemBinary(asset string, files map[string][]byte) (base, hexsum string, ok bool) {
	name, ok := stemSumName(asset)
	if !ok {
		return "", "", false
	}
	body, ok := files[name]
	if !ok {
		return "", "", false
	}
	return oneBinarySum(body)
}

func oneBinarySum(body []byte) (base, hexsum string, ok bool) {
	var foundBase, foundHex string
	n := 0
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 || !isHex64(f[0]) {
			continue
		}
		name := strings.TrimPrefix(f[len(f)-1], "*")
		b := filepath.Base(name)
		if b == "" || b == "." || b == ".." {
			continue
		}
		n++
		foundBase = b
		foundHex = strings.ToLower(f[0])
	}
	if n != 1 {
		return "", "", false
	}
	return foundBase, foundHex, true
}

func verifyExtracted(archivePath, base, hexsum string) error {
	tmp, err := os.MkdirTemp("", "dotfiles-tools-sum-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := UnpackArchive(archivePath, tmp); err != nil {
		return err
	}
	match, n, err := findRegularBase(tmp, base)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("找不到 %s", base)
	}
	return VerifyFile(match, hexsum)
}

func findRegularBase(root, base string) (string, int, error) {
	var match string
	n := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Name() == base {
			n++
			match = path
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	if n != 1 {
		return "", n, nil
	}
	return match, 1, nil
}

func hashFor(asset, checksumName string, body []byte) (string, bool) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", false
	}
	fields := strings.Fields(text)
	if len(fields) == 1 && isHex64(fields[0]) && checksumName == asset+".sha256" {
		return strings.ToLower(fields[0]), true
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 || !isHex64(f[0]) {
			continue
		}
		name := strings.TrimPrefix(f[len(f)-1], "*")
		if name == asset || filepath.Base(name) == asset {
			return strings.ToLower(f[0]), true
		}
	}
	return "", false
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// VerifyFile compares path to want and deletes path when they differ.
func VerifyFile(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	f.Close()
	if copyErr != nil {
		return copyErr
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		os.Remove(path)
		return fmt.Errorf("SHA-256 不符")
	}
	return nil
}
