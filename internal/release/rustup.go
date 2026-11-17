package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func RustTriple(goos, goarch, uname, libc string) (string, error) {
	if libc == "" {
		libc = "gnu"
	}
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", nil
		case "arm64":
			return "aarch64-apple-darwin", nil
		default:
			return "", unsupported(goarch)
		}
	case "linux":
		kind := "gnu"
		if libc == "musl" {
			kind = "musl"
		}
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-" + kind, nil
		case "386":
			return "i686-unknown-linux-" + kind, nil
		case "arm64":
			return "aarch64-unknown-linux-" + kind, nil
		case "arm":
			switch uname {
			case "armv6l":
				if kind == "musl" {
					return "arm-unknown-linux-musleabihf", nil
				}
				return "arm-unknown-linux-gnueabihf", nil
			case "armv7l":
				if kind == "musl" {
					return "armv7-unknown-linux-musleabihf", nil
				}
				return "armv7-unknown-linux-gnueabihf", nil
			default:
				return "", unsupported(goarch)
			}
		case "ppc64le":
			return "powerpc64le-unknown-linux-" + kind, nil
		case "s390x":
			return "s390x-unknown-linux-" + kind, nil
		case "riscv64":
			return "riscv64gc-unknown-linux-" + kind, nil
		case "loong64":
			return "loongarch64-unknown-linux-" + kind, nil
		default:
			return "", unsupported(goarch)
		}
	default:
		return "", unsupported(goarch)
	}
}

func RustupArgv(initPath string) []string {
	return []string{initPath, "-y", "--no-modify-path", "--default-toolchain", "stable"}
}

func (c *Client) PrepareRustup(ctx context.Context, q AssetQuery, dir string) ([]string, error) {
	triple, err := RustTriple(q.GOOS, q.GOARCH, q.UnameM, q.Libc)
	if err != nil {
		return nil, err
	}
	base := "https://static.rust-lang.org/rustup/dist/" + triple + "/rustup-init"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	bin := filepath.Join(dir, "rustup-init")
	sumPath := filepath.Join(dir, "rustup-init.sha256")
	if err := c.Download(ctx, base, bin); err != nil {
		return nil, err
	}
	if err := c.Download(ctx, base+".sha256", sumPath); err != nil {
		os.Remove(bin)
		return nil, err
	}
	body, err := os.ReadFile(sumPath)
	if err != nil {
		return nil, err
	}
	hexsum, ok := hashFor("rustup-init", "rustup-init.sha256", body)
	if !ok {
		os.Remove(bin)
		return nil, fmt.Errorf("checksum 沒有 rustup-init 的列")
	}
	if err := VerifyFile(bin, hexsum); err != nil {
		return nil, err
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		return nil, err
	}
	return RustupArgv(bin), nil
}

// DetectLibc reports musl when os-release ID is alpine or exists reports a musl loader.
// exists is invoked with the glob /lib/ld-musl*.
func DetectLibc(osID string, exists func(string) bool) string {
	if osID == "alpine" {
		return "musl"
	}
	if exists != nil && exists("/lib/ld-musl*") {
		return "musl"
	}
	return "gnu"
}

func LibcExists(pattern string) bool {
	matches, err := filepath.Glob(pattern)
	return err == nil && len(matches) > 0
}
