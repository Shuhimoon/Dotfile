package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Asset struct {
	Name string
	URL  string
}

type GitRelease struct {
	Tag    string
	Assets []Asset
}

type Client struct {
	Meta HTTPDoer
	File HTTPDoer

	mu    sync.Mutex
	cache map[string]GitRelease
}

func NewClients() *Client {
	return &Client{
		Meta: &http.Client{Timeout: 30 * time.Second},
		File: &http.Client{Timeout: 0},
	}
}

func (c *Client) Latest(ctx context.Context, repo string) (GitRelease, error) {
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]GitRelease{}
	}
	if rel, ok := c.cache[repo]; ok {
		c.mu.Unlock()
		return rel, nil
	}
	c.mu.Unlock()
	if c.Meta == nil {
		return GitRelease{}, fmt.Errorf("沒有中繼資料 HTTP client")
	}
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return GitRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dotfiles-tools")
	resp, err := c.Meta.Do(req)
	if err != nil {
		return GitRelease{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return GitRelease{}, err
	}
	if resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(string(body)), "rate limit") {
		return GitRelease{}, fmt.Errorf("GitHub API rate limit")
	}
	if resp.StatusCode != http.StatusOK {
		return GitRelease{}, fmt.Errorf("GitHub HTTP %d", resp.StatusCode)
	}
	var raw struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return GitRelease{}, err
	}
	rel := GitRelease{Tag: raw.TagName}
	for _, a := range raw.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL})
	}
	c.mu.Lock()
	c.cache[repo] = rel
	c.mu.Unlock()
	return rel, nil
}

func (c *Client) Download(ctx context.Context, url, dest string) error {
	if c.File == nil {
		return fmt.Errorf("沒有檔案 HTTP client")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dotfiles-tools")
	resp, err := c.File.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下載 HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(dest)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(dest)
		return closeErr
	}
	return nil
}

func (c *Client) Install(ctx context.Context, repo, home string, q AssetQuery) error {
	if repo == ZigRepo {
		return c.InstallZig(ctx, home, q)
	}
	rel, err := c.Latest(ctx, repo)
	if err != nil {
		return err
	}
	if q.Version == "" {
		q.Version = strings.TrimPrefix(rel.Tag, "v")
	}
	spec, ok := SpecFor(repo)
	if !ok {
		return fmt.Errorf("未知的 repo %s", repo)
	}
	names := make([]string, 0, len(rel.Assets))
	byName := map[string]string{}
	for _, a := range rel.Assets {
		names = append(names, a.Name)
		byName[a.Name] = a.URL
	}
	asset, err := Match(repo, names, q)
	if err != nil {
		return err
	}
	csums := ChecksumNames(asset, names)
	if len(csums) == 0 {
		return fmt.Errorf("沒有 checksum：%s", asset)
	}
	tmp, err := os.MkdirTemp("", "dotfiles-tools-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	files := map[string][]byte{}
	for _, name := range csums {
		path := filepath.Join(tmp, "sum-"+filepath.Base(name))
		if err := c.Download(ctx, byName[name], path); err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[name] = b
	}
	hexsum, sumErr := ExpectedSHA256(asset, files)
	_, _, stemOK := stemBinary(asset, files)
	if sumErr != nil && !stemOK {
		return sumErr
	}
	assetPath := filepath.Join(tmp, asset)
	if err := c.Download(ctx, byName[asset], assetPath); err != nil {
		return err
	}
	if sumErr == nil {
		if err := VerifyFile(assetPath, hexsum); err != nil {
			return err
		}
	} else if err := VerifyStemBinary(assetPath, asset, files); err != nil {
		return err
	}
	return InstallLayout(assetPath, home, spec)
}
