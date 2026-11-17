package pm

import "fmt"

// BuildRemove is the uninstall argv for pkgs, in the same style as BuildInstall.
// Privilege (sudo) is added by WithPrivilege, exactly as for installs.
func BuildRemove(manager string, pkgs []string) ([]string, error) {
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
		argv = append([]string{"apt-get", "remove", "-y"}, pkgs...)
	case "dnf":
		argv = append([]string{"dnf", "remove", "-y"}, pkgs...)
	case "yum":
		argv = append([]string{"yum", "remove", "-y"}, pkgs...)
	case "pacman":
		argv = append([]string{"pacman", "-R", "--noconfirm"}, pkgs...)
	case "zypper":
		argv = append([]string{"zypper", "--non-interactive", "remove"}, pkgs...)
	case "apk":
		argv = append([]string{"apk", "del"}, pkgs...)
	case "xbps":
		argv = append([]string{"xbps-remove", "-y"}, pkgs...)
	case "brew":
		argv = append([]string{"brew", "uninstall"}, pkgs...)
	default:
		return nil, fmt.Errorf("未知的套件管理器 %s", manager)
	}
	if err := ValidateArgv(argv); err != nil {
		return nil, err
	}
	return argv, nil
}
