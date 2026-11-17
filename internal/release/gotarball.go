package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/shuhi/dotfiles/internal/pm"
)

type goFile struct {
	Filename string
	SHA256   string
	URL      string
}

func SelectGoFile(body []byte, goos, goarch string) (goFile, error) {
	arch, err := goArch(goos, goarch)
	if err != nil {
		return goFile{}, err
	}
	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
		Files   []struct {
			Filename string `json:"filename"`
			SHA256   string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return goFile{}, err
	}
	for _, rel := range releases {
		if !rel.Stable {
			continue
		}
		want := rel.Version + "." + goos + "-" + arch + ".tar.gz"
		for _, f := range rel.Files {
			if f.Filename == want {
				if f.SHA256 == "" {
					return goFile{}, fmt.Errorf("go tarball 沒有 sha256")
				}
				return goFile{Filename: f.Filename, SHA256: f.SHA256, URL: "https://go.dev/dl/" + f.Filename}, nil
			}
		}
		return goFile{}, fmt.Errorf("找不到資產：%s", want)
	}
	return goFile{}, fmt.Errorf("找不到 stable go release")
}

func goArch(goos, goarch string) (string, error) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64", "arm64", "386", "ppc64le", "s390x", "riscv64", "loong64":
			return goarch, nil
		case "arm":
			return "armv6l", nil
		default:
			return "", unsupported(goarch)
		}
	case "darwin":
		switch goarch {
		case "amd64", "arm64":
			return goarch, nil
		default:
			return "", unsupported(goarch)
		}
	default:
		return "", unsupported(goarch)
	}
}

func (c *Client) InstallGo(ctx context.Context, q AssetQuery, euid int, destRoot string) error {
	if euid != 0 {
		return fmt.Errorf("%s", pm.GoNoRootMessage)
	}
	if c.Meta == nil {
		return fmt.Errorf("沒有中繼資料 HTTP client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://go.dev/dl/?mode=json", nil)
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
		return fmt.Errorf("go.dev HTTP %d", resp.StatusCode)
	}
	file, err := SelectGoFile(body, q.GOOS, q.GOARCH)
	if err != nil {
		return err
	}
	dest := filepath.Join(destRoot, "go")
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("%s 已存在，不覆蓋", dest)
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
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}
	return UnpackArchive(archive, destRoot)
}
