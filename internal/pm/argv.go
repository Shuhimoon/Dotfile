package pm

import (
	"fmt"
	"strings"
)

func BuildInstall(manager string, pkgs []string) ([]string, error) {
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("套件清單是空的")
	}
	for _, p := range pkgs {
		if err := rejectMeta(p); err != nil {
			return nil, err
		}
	}
	var argv []string
	switch manager {
	case "apt":
		argv = append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, pkgs...)
	case "dnf":
		argv = append([]string{"dnf", "install", "-y"}, pkgs...)
	case "yum":
		argv = append([]string{"yum", "install", "-y"}, pkgs...)
	case "pacman":
		argv = append([]string{"pacman", "-S", "--needed", "--noconfirm"}, pkgs...)
	case "zypper":
		argv = append([]string{"zypper", "--non-interactive", "install", "--no-recommends"}, pkgs...)
	case "apk":
		argv = append([]string{"apk", "add"}, pkgs...)
	case "xbps":
		argv = append([]string{"xbps-install", "-y", "-S"}, pkgs...)
	case "brew":
		argv = append([]string{"brew", "install"}, pkgs...)
	default:
		return nil, fmt.Errorf("未知的套件管理器 %s", manager)
	}
	if err := ValidateArgv(argv); err != nil {
		return nil, err
	}
	return argv, nil
}

func BuildUpdate(manager string) ([]string, error) {
	var argv []string
	switch manager {
	case "apt":
		argv = []string{"apt-get", "update"}
	case "pacman":
		argv = []string{"pacman", "-Sy", "--noconfirm"}
	case "zypper":
		argv = []string{"zypper", "--non-interactive", "refresh"}
	case "apk":
		argv = []string{"apk", "update"}
	case "dnf", "yum", "xbps", "brew":
		return nil, nil
	default:
		return nil, fmt.Errorf("未知的套件管理器 %s", manager)
	}
	if err := ValidateArgv(argv); err != nil {
		return nil, err
	}
	return argv, nil
}

func WithPrivilege(argv []string, need bool, euid int) []string {
	if argv == nil {
		return nil
	}
	out := append([]string(nil), argv...)
	if need && euid != 0 {
		return append([]string{"sudo"}, out...)
	}
	return out
}

func NeedsPrivilege(manager string) bool {
	return manager != "brew"
}

func rejectMeta(s string) error {
	if s == "" || strings.ContainsAny(s, " \t;|&$\n\r") || strings.Contains(s, "`") {
		return fmt.Errorf("非法套件名稱 %q", s)
	}
	if s == "sh" || s == "bash" || s == "-c" {
		return fmt.Errorf("argv 禁止 shell")
	}
	return nil
}

func ValidateArgv(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("空的 argv")
	}
	idx := 0
	if argv[0] == "sudo" {
		if len(argv) < 2 {
			return fmt.Errorf("sudo 缺少命令")
		}
		idx = 1
	}
	switch argv[idx] {
	case "apt-get", "dnf", "yum", "pacman", "zypper", "apk", "xbps-install", "xbps-remove", "brew", "xcode-select":
	case "scoop":
		return ValidateScoopArgv(argv)
	default:
		return fmt.Errorf("不允許的命令 %s", argv[idx])
	}
	for _, a := range argv {
		if a == "sh" || a == "bash" || a == "-c" {
			return fmt.Errorf("argv 禁止 shell：%s", a)
		}
		if strings.ContainsAny(a, " \t;|&$\n\r") || strings.Contains(a, "`") {
			return fmt.Errorf("argv 含有非法字元")
		}
	}
	return nil
}
