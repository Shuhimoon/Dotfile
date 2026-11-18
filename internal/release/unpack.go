package release

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/safepath"
	"github.com/ulikunitz/xz"
)

const (
	maxUnpackBytes = 1 << 30 // the zig tarball unpacks to several hundred MiB
	maxUnpackFiles = 100000  // zig ships tens of thousands of libc headers
)

func UnpackArchive(archivePath, dest string) error {
	r, closeFn, err := openTar(archivePath)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	var files int
	var total int64
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		files++
		if files > maxUnpackFiles {
			return fmt.Errorf("檔案數超過 %d", maxUnpackFiles)
		}
		name := hdr.Name
		if name == "" || strings.ContainsRune(name, 0) || filepath.IsAbs(name) {
			return fmt.Errorf("拒絕路徑：%s", hdr.Name)
		}
		clean := filepath.Clean(name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("拒絕路徑：%s", hdr.Name)
		}
		target := filepath.Join(dest, clean)
		rel, err := filepath.Rel(dest, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("拒絕路徑：%s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if hdr.Size < 0 || total > maxUnpackBytes-hdr.Size {
				return fmt.Errorf("解壓超過 512 MiB")
			}
			total += hdr.Size
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			if err := writeReg(target, io.LimitReader(r, hdr.Size), mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if linkEscapes(dest, clean, hdr.Linkname) {
				return fmt.Errorf("拒絕 symlink 指向目標之外：%s", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("不支援的 tar 類型 %q", hdr.Name)
		}
	}
}

func linkEscapes(dest, entry, linkTarget string) bool {
	if linkTarget == "" || filepath.IsAbs(linkTarget) {
		return true
	}
	base := filepath.Join(dest, filepath.Dir(entry))
	resolved := filepath.Clean(filepath.Join(base, linkTarget))
	rel, err := filepath.Rel(dest, resolved)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func writeReg(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	cerr := f.Close()
	if err != nil {
		return err
	}
	return cerr
}

func openTar(path string) (*tar.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	var reader io.Reader = f
	var closers []func()
	closers = append(closers, func() { f.Close() })
	switch {
	case strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		reader = gz
		closers = append(closers, func() { gz.Close() })
	case strings.HasSuffix(path, ".tar.xz"):
		xzr, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		reader = xzr
	default:
		f.Close()
		return nil, nil, fmt.Errorf("只接受 .tar.gz 與 .tar.xz")
	}
	return tar.NewReader(reader), func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}, nil
}

func InstallLayout(archivePath, home string, spec Spec) error {
	if home == "" || !filepath.IsAbs(home) {
		return fmt.Errorf("home 必須是絕對路徑")
	}
	link := filepath.Join(home, ".local", "bin", spec.BinaryName)
	if err := safepath.RejectEscapingSymlink(filepath.Dir(link), home); err != nil {
		return err
	}
	// A tree layout owns its ~/.local/bin symlink (PlaceSymlink replaces it on
	// a reinstall); the other layouts write a file there and refuse a symlink.
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink != 0 && spec.Layout != LayoutTree {
		return fmt.Errorf("拒絕 symlink：%s", link)
	}
	tmp, err := os.MkdirTemp("", "dotfiles-tools-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := UnpackArchive(archivePath, tmp); err != nil {
		return err
	}
	switch spec.Layout {
	case LayoutBinDirect:
		return installDirect(tmp, link, spec.BinaryName)
	case LayoutBinInTopDir:
		return installTop(tmp, link, spec.BinaryName)
	case LayoutTree:
		return installTree(tmp, home, link, spec)
	default:
		return fmt.Errorf("未知的 layout")
	}
}

func installDirect(tmp, link, binary string) error {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	var regs []string
	named := ""
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		regs = append(regs, e.Name())
		if e.Name() == binary {
			named = e.Name()
		}
	}
	src := named
	if src == "" {
		if len(regs) != 1 {
			return fmt.Errorf("根目錄沒有唯一的 %s", binary)
		}
		src = regs[0]
	}
	return copyExec(filepath.Join(tmp, src), link)
}

func installTop(tmp, link, binary string) error {
	top, err := singleTop(tmp)
	if err != nil {
		return err
	}
	src := filepath.Join(tmp, top, binary)
	info, err := os.Lstat(src)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("找不到 %s/%s", top, binary)
	}
	return copyExec(src, link)
}

func installTree(tmp, home, link string, spec Spec) error {
	if spec.ShareSubdir == "" || spec.SymlinkRel == "" {
		return fmt.Errorf("LayoutTree 缺少 ShareSubdir 或 SymlinkRel")
	}
	top, err := singleTop(tmp)
	if err != nil {
		return err
	}
	dest := filepath.Join(home, ".local", "share", spec.ShareSubdir, top)
	if err := safepath.RejectEscapingSymlink(dest, home); err != nil {
		return err
	}
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("拒絕 symlink：%s", dest)
		}
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	if err := copyTree(filepath.Join(tmp, top), dest); err != nil {
		return err
	}
	if spec.ShareSubdir == "fastfetch" {
		share := filepath.Join(dest, "usr", "share", "fastfetch")
		info, err := os.Lstat(share)
		if err != nil || !info.IsDir() {
			os.RemoveAll(dest)
			return fmt.Errorf("找不到 usr/share/fastfetch")
		}
	}
	target := filepath.Join(dest, spec.SymlinkRel)
	if _, err := os.Lstat(target); err != nil {
		return fmt.Errorf("找不到 %s", spec.SymlinkRel)
	}
	_, err = PlaceSymlink(link, target, true)
	return err
}

func singleTop(tmp string) (string, error) {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 {
		return "", fmt.Errorf("需要恰好一個頂層目錄，得到 %d 個", len(dirs))
	}
	return dirs[0], nil
}

func copyExec(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".bin-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	ok = true
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, out)
		}
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		mode := info.Mode().Perm()
		if mode&0o111 != 0 {
			mode = 0o755
		} else {
			mode = 0o644
		}
		return copyFile(path, out, mode)
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// PlaceSymlink creates link → target. replaceSymlink replaces an existing symlink.
// A regular file is never overwritten. An already-correct symlink is a no-op.
func PlaceSymlink(link, target string, replaceSymlink bool) (bool, error) {
	info, err := os.Lstat(link)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("拒絕覆蓋既有檔案：%s", link)
		}
		got, err := os.Readlink(link)
		if err != nil {
			return false, err
		}
		if got == target {
			return false, nil
		}
		if !replaceSymlink {
			return false, fmt.Errorf("symlink 指向別處：%s", link)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(link), ".sl-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	tmp.Close()
	os.Remove(tmpName)
	if err := os.Symlink(target, tmpName); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, link); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	return true, nil
}
