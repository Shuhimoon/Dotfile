// Package sshkey picks or stores the private key GitHub should use and points
// ~/.ssh/config at it through a managed block. Key contents are never logged,
// printed, or returned in errors.
package sshkey

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	BlockBegin = "# >>> dotfiles:github >>>"
	BlockEnd   = "# <<< dotfiles:github <<<"
)

// Layout says where keys live and how the config names them.
// Mac/Linux: ~/.ssh/keys/<name>, IdentityFile ~/.ssh/keys/<name>.
// Windows:   ~/.ssh/<name>,      IdentityFile ~/.ssh/<name>.
type Layout struct {
	SSHDir   string // ~/.ssh
	KeyDir   string // where keys are listed and saved
	IdentRel string // "~/.ssh/keys/" or "~/.ssh/"
}

func LayoutFor(goos, home string) Layout {
	ssh := filepath.Join(home, ".ssh")
	if goos == "windows" {
		return Layout{SSHDir: ssh, KeyDir: ssh, IdentRel: "~/.ssh/"}
	}
	return Layout{SSHDir: ssh, KeyDir: filepath.Join(ssh, "keys"), IdentRel: "~/.ssh/keys/"}
}

func (l Layout) ConfigPath() string { return filepath.Join(l.SSHDir, "config") }

// Identity is the IdentityFile value for a key name.
func (l Layout) Identity(name string) string { return l.IdentRel + name }

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// reserved names are the non-key files that live next to keys in ~/.ssh.
func reserved(name string) bool {
	low := strings.ToLower(name)
	if strings.HasSuffix(low, ".pub") || strings.HasPrefix(low, "known_hosts") ||
		strings.HasPrefix(low, "authorized_keys") || strings.HasPrefix(low, "config") ||
		strings.HasPrefix(low, "environment") || strings.Contains(low, ".bak") {
		return true
	}
	return false
}

// ValidName accepts a plain file name: letters, digits, '.', '_', '-', not
// starting with '.' or '-', no slashes, not a .pub / config / known_hosts name.
func ValidName(name string) error {
	if name == "" {
		return errors.New("請輸入鑰匙名稱")
	}
	if !nameRe.MatchString(name) {
		return errors.New("鑰匙名稱只能用英數字、. _ -，開頭須為英數字，最多 64 字")
	}
	if reserved(name) {
		return fmt.Errorf("不能用 %s 當鑰匙名稱", name)
	}
	return nil
}

// LooksPrivate reports whether b starts like a PEM/OpenSSH private key.
func LooksPrivate(b []byte) bool {
	s := strings.TrimLeft(string(b), " \t\r\n")
	if !strings.HasPrefix(s, "-----BEGIN ") {
		return false
	}
	line, _, _ := strings.Cut(s, "\n")
	line = strings.TrimSpace(line)
	return strings.HasSuffix(line, "PRIVATE KEY-----")
}

// ListKeys returns the private key file names in dir, sorted. Missing dir is
// not an error. Only regular files whose content starts with a private key
// header count; .pub, known_hosts, config and friends are skipped.
func ListKeys(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || reserved(name) || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
			continue
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		head := make([]byte, 128)
		n, _ := io.ReadFull(f, head)
		f.Close()
		if LooksPrivate(head[:n]) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Normalize trims the pasted text, turns CRLF into LF and ends it with one newline.
func Normalize(key []byte) []byte {
	s := strings.ReplaceAll(string(key), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []byte(s + "\n")
}

// Check validates a pasted private key. Passphrase-protected keys are
// accepted as they are; the error never includes key material.
func Check(key []byte) error {
	if !LooksPrivate(key) {
		return errors.New("不是私鑰：要以 -----BEGIN … PRIVATE KEY----- 開頭")
	}
	if _, err := ssh.ParseRawPrivateKey(key); err != nil {
		var pm *ssh.PassphraseMissingError
		if errors.As(err, &pm) {
			return nil
		}
		return errors.New("無法解析私鑰，請確認貼上的是完整的私鑰")
	}
	return nil
}

// SaveKey writes key to KeyDir/<name> with 0600 (dirs 0700). An existing
// file of that name is never overwritten.
func (l Layout) SaveKey(name string, key []byte) (string, error) {
	if err := ValidName(name); err != nil {
		return "", err
	}
	body := Normalize(key)
	defer clear(body)
	if err := Check(body); err != nil {
		return "", err
	}
	for _, d := range []string{l.SSHDir, l.KeyDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
		_ = os.Chmod(d, 0o700)
	}
	path := filepath.Join(l.KeyDir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("已經有叫 %s 的檔案，請換一個名稱", name)
	}
	if err != nil {
		return "", err
	}
	_, werr := f.Write(body)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		os.Remove(path)
		return "", errors.Join(werr, cerr)
	}
	restrict(path)
	return path, nil
}

// Block is the managed ~/.ssh/config block for identity.
func Block(identity string) string {
	return BlockBegin + "\n" +
		"Host github.com\n" +
		"  HostName github.com\n" +
		"  User git\n" +
		"  IdentityFile " + identity + "\n" +
		BlockEnd + "\n"
}

// strip removes every managed block and reports whether one was there.
func strip(s string) (string, bool) {
	found := false
	for {
		i := strings.Index(s, BlockBegin)
		if i < 0 {
			return s, found
		}
		j := strings.Index(s[i:], BlockEnd)
		if j < 0 {
			return s, found
		}
		end := i + j + len(BlockEnd)
		if end < len(s) && s[end] == '\n' {
			end++
		}
		s = s[:i] + s[end:]
		found = true
	}
}

// HasUnmanagedGitHub reports a "Host" line outside the managed block that
// matches github.com.
func HasUnmanagedGitHub(config string) bool {
	rest, _ := strip(config)
	for _, line := range strings.Split(rest, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || !strings.EqualFold(f[0], "host") {
			continue
		}
		for _, h := range f[1:] {
			if strings.EqualFold(h, "github.com") {
				return true
			}
		}
	}
	return false
}

// Render puts the managed block first (ssh uses the first value it finds, so
// it wins over an older "Host github.com"), then the rest of the file.
func Render(config, identity string) string {
	rest, _ := strip(config)
	rest = strings.TrimLeft(rest, "\n")
	if rest == "" {
		return Block(identity)
	}
	return Block(identity) + "\n" + rest
}

type ApplyResult struct {
	Path      string
	Changed   bool
	Backup    string // set when the file was backed up before its first managed change
	Unmanaged bool   // an older Host github.com outside the block exists
}

// Apply writes the managed block into ~/.ssh/config (0600, dir 0700). The file
// is backed up to config.bak.<time> before the first change that adds the block.
func (l Layout) Apply(identity string, now time.Time) (ApplyResult, error) {
	res := ApplyResult{Path: l.ConfigPath()}
	if err := os.MkdirAll(l.SSHDir, 0o700); err != nil {
		return res, err
	}
	old, err := os.ReadFile(res.Path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return res, err
	}
	res.Unmanaged = HasUnmanagedGitHub(string(old))
	next := Render(string(old), identity)
	if existed && next == string(old) {
		return res, nil
	}
	_, hadBlock := strip(string(old))
	if existed && !hadBlock && len(bytes.TrimSpace(old)) > 0 {
		res.Backup = res.Path + ".bak." + now.UTC().Format("20060102T150405Z")
		if err := os.WriteFile(res.Backup, old, 0o600); err != nil {
			return res, err
		}
	}
	if err := writeAtomic(res.Path, []byte(next)); err != nil {
		return res, err
	}
	res.Changed = true
	return res, nil
}

// Current returns the IdentityFile in the managed block, or "".
func Current(configPath string) string {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	s := string(b)
	i := strings.Index(s, BlockBegin)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], BlockEnd)
	if j < 0 {
		return ""
	}
	for _, line := range strings.Split(s[i:i+j], "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.EqualFold(f[0], "IdentityFile") {
			return f[1]
		}
	}
	return ""
}

// RemoveBlock drops the managed block; key files are kept.
func RemoveBlock(configPath string) (bool, error) {
	b, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	next, found := strip(string(b))
	if !found {
		return false, nil
	}
	return true, writeAtomic(configPath, []byte(strings.TrimLeft(next, "\n")))
}

func writeAtomic(path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(body)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(name)
		return errors.Join(werr, cerr)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	restrict(path)
	return nil
}
