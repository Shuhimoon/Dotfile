package pm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shuhi/dotfiles/internal/platform"
)

var (
	ErrBrewMissing      = errors.New("darwin 需要 Homebrew（brew），但 PATH 中找不到 brew")
	ErrManagerMissing   = errors.New("不支援的套件管理程式，本次不會安裝任何套件")
	ErrManagerAmbiguous = errors.New("套件管理器歧義")
)

func DetectManager(goos string, osr platform.OSRelease, lookup func(bin string) bool) (string, error) {
	if lookup == nil {
		lookup = func(string) bool { return false }
	}
	switch goos {
	case "windows":
		// Scoop is bootstrapped by the first install run when it is missing.
		return Scoop, nil
	case "darwin":
		if !lookup("brew") {
			return "", ErrBrewMissing
		}
		return "brew", nil
	case "linux":
	default:
		return "", fmt.Errorf("不支援的 GOOS=%s：v1 只支援 linux 與 darwin", goos)
	}

	var candidates []string
	if id := strings.ToLower(strings.TrimSpace(osr.ID)); id != "" {
		candidates = append(candidates, id)
	}
	for _, tok := range strings.Fields(osr.IDLike) {
		candidates = append(candidates, strings.ToLower(tok))
	}
	var fam, distro string
	for _, c := range candidates {
		if f := familyOf(c); f != "" {
			fam = f
			distro = c
			break
		}
	}
	if fam != "" {
		return fromFamily(fam, distro, lookup)
	}
	return probeManagers(lookup)
}

func familyOf(id string) string {
	switch id {
	case "debian", "ubuntu", "linuxmint", "pop", "elementary", "raspbian", "kali":
		return "apt"
	case "fedora":
		return "fedora"
	case "rhel", "centos", "rocky", "almalinux", "ol", "amzn", "scientific":
		return "rhel"
	case "arch", "manjaro", "endeavouros", "garuda", "arcolinux":
		return "pacman"
	case "opensuse", "opensuse-tumbleweed", "opensuse-leap", "opensuse-slowroll", "sles":
		return "zypper"
	case "alpine":
		return "apk"
	case "void":
		return "xbps"
	default:
		return ""
	}
}

func fromFamily(fam, distro string, lookup func(string) bool) (string, error) {
	switch fam {
	case "fedora":
		if lookup("dnf") {
			return "dnf", nil
		}
		return "", fmt.Errorf("發行版 %s 缺少二進位 dnf", distro)
	case "rhel":
		if lookup("dnf") {
			return "dnf", nil
		}
		if lookup("yum") {
			return "yum", nil
		}
		return "", fmt.Errorf("發行版 %s 缺少二進位 dnf", distro)
	case "apt":
		return need("apt", "apt-get", distro, lookup)
	case "pacman":
		return need("pacman", "pacman", distro, lookup)
	case "zypper":
		return need("zypper", "zypper", distro, lookup)
	case "apk":
		return need("apk", "apk", distro, lookup)
	case "xbps":
		return need("xbps", "xbps-install", distro, lookup)
	default:
		return "", fmt.Errorf("未知的套件家族 %s", fam)
	}
}

func need(id, bin, distro string, lookup func(string) bool) (string, error) {
	if lookup(bin) {
		return id, nil
	}
	return "", fmt.Errorf("發行版 %s 缺少二進位 %s", distro, bin)
}

func probeManagers(lookup func(string) bool) (string, error) {
	var found []string
	var id string
	for _, m := range managerOrder {
		if m == "brew" {
			continue
		}
		bin := binaries[m]
		if lookup(bin) {
			found = append(found, bin)
			id = m
		}
	}
	switch len(found) {
	case 0:
		return "", ErrManagerMissing
	case 1:
		return id, nil
	default:
		return "", fmt.Errorf("%w，找到多個二進位：%s", ErrManagerAmbiguous, strings.Join(found, ", "))
	}
}

// CheckExplicit validates a --pm override. Family mapping is skipped.
func CheckExplicit(id string, lookup func(string) bool) error {
	bin, ok := Binary(id)
	if !ok {
		return fmt.Errorf("不支援的套件管理器 %s", id)
	}
	if lookup == nil || !lookup(bin) {
		return fmt.Errorf("套件管理器 %s 的二進位 %s 不存在", id, bin)
	}
	return nil
}
