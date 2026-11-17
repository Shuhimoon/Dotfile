package pm

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shuhi/dotfiles/internal/catalog"
)

// Windows support: Scoop is the only package manager. It installs per user
// under %USERPROFILE%\scoop (or %SCOOP%) without admin rights, so there is no
// sudo, no update step, and no shell rc files on Windows.

const Scoop = "scoop"

// ScoopBootstrapScript is the official installer (https://get.scoop.sh). It
// must run as a normal user; Scoop refuses an elevated shell by default.
const ScoopBootstrapScript = "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force; irm get.scoop.sh | iex"

// ScoopBootstrapArgv runs the installer through Windows PowerShell.
func ScoopBootstrapArgv() []string {
	return []string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ScoopBootstrapScript}
}

// ScoopItem is one Windows catalog item.
type ScoopItem struct {
	ID      catalog.ID
	Bucket  string // "" for main
	Package string // argument to scoop install / uninstall
	Binary  string // shim name, without .exe
}

var scoopItems = map[catalog.ID]ScoopItem{
	"wezterm": {ID: "wezterm", Bucket: "extras", Package: "extras/wezterm", Binary: "wezterm"},
	"python":  {ID: "python", Package: "python", Binary: "python"},
	"git":     {ID: "git", Package: "git", Binary: "git"},
}

func ScoopFor(id catalog.ID) (ScoopItem, bool) {
	it, ok := scoopItems[id]
	return it, ok
}

// UninstallName is the app name scoop uninstall wants (no bucket prefix).
func (it ScoopItem) UninstallName() string {
	if _, name, ok := strings.Cut(it.Package, "/"); ok {
		return name
	}
	return it.Package
}

func (it ScoopItem) InstallArgv() []string { return []string{"scoop", "install", it.Package} }
func (it ScoopItem) RemoveArgv() []string  { return []string{"scoop", "uninstall", it.UninstallName()} }

func ScoopBucketArgv(bucket string) []string { return []string{"scoop", "bucket", "add", bucket} }

// ScoopRoot is %SCOOP% when set, else <home>\scoop.
func ScoopRoot(home, envScoop string) string {
	if strings.TrimSpace(envScoop) != "" {
		return envScoop
	}
	return filepath.Join(home, "scoop")
}

// ScoopShim is the shim scoop puts on PATH for bin.
func ScoopShim(root, bin string) string {
	return filepath.Join(root, "shims", bin+".exe")
}

// ScoopCommands are the shims of scoop itself.
func ScoopCommands(root string) []string {
	return []string{filepath.Join(root, "shims", "scoop.cmd"), filepath.Join(root, "shims", "scoop.ps1")}
}

// ValidateScoopArgv allows only "scoop install|uninstall <app>" and
// "scoop bucket add <bucket>" with plain arguments.
func ValidateScoopArgv(argv []string) error {
	if len(argv) < 3 || argv[0] != "scoop" {
		return fmt.Errorf("不允許的 scoop argv")
	}
	switch {
	case (argv[1] == "install" || argv[1] == "uninstall") && len(argv) == 3:
	case argv[1] == "bucket" && len(argv) == 4 && argv[2] == "add":
	default:
		return fmt.Errorf("不允許的 scoop argv")
	}
	for _, a := range argv[1:] {
		if !scoopArgRe.MatchString(a) {
			return fmt.Errorf("argv 含有非法字元")
		}
	}
	return nil
}

var scoopArgRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_./-]*$`)
