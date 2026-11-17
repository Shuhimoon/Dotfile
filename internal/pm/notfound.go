package pm

import (
	"fmt"
	"strings"
)

type NotFoundError struct {
	Manager string
	Output  string
}

func (e *NotFoundError) Error() string {
	return "套件不存在"
}

func ClassifyFailure(manager string, exitCode int, output string) error {
	if exitCode == 0 {
		return nil
	}
	if strings.Contains(output, "sudo: a password is required") || strings.Contains(output, "a terminal is required") {
		return fmt.Errorf("sudo 失敗")
	}
	if notFound(manager, exitCode, output) {
		return &NotFoundError{Manager: manager, Output: output}
	}
	return fmt.Errorf("命令失敗 exit %d", exitCode)
}

func notFound(manager string, code int, output string) bool {
	switch manager {
	case "apt":
		return code == 100 && (strings.Contains(output, "Unable to locate package") || strings.Contains(output, "has no installation candidate"))
	case "dnf":
		return code == 1 && (strings.Contains(output, "No match for argument") || strings.Contains(output, "Unable to find a match"))
	case "yum":
		if code != 1 {
			return false
		}
		if strings.Contains(output, "No match for argument") {
			return true
		}
		i := strings.Index(output, "No package")
		return i >= 0 && strings.Contains(output[i:], "available")
	case "pacman":
		return code == 1 && strings.Contains(output, "target not found")
	case "zypper":
		return code == 104
	case "apk":
		low := strings.ToLower(output)
		return code != 0 && (strings.Contains(low, "no such package") || strings.Contains(low, "unable to select packages"))
	case "xbps":
		return code != 0 && strings.Contains(output, "not found in repository pool")
	case "brew":
		return code == 1 && (strings.Contains(output, "No available formula") || strings.Contains(output, "No formulae found"))
	default:
		return false
	}
}
