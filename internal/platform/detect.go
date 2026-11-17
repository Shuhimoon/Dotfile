package platform

import (
	"os"
	"os/exec"
	"strings"
)

func DetectWSL(envWSLDistro string, procVersion string) bool {
	if strings.TrimSpace(envWSLDistro) != "" {
		return true
	}
	low := strings.ToLower(procVersion)
	return strings.Contains(low, "microsoft") || strings.Contains(low, "wsl")
}

// Info is the header aggregate. Manager is the pm ID, empty when unknown.
type Info struct {
	OS      OSInfo
	Arch    ArchInfo
	Manager string
}

func LoadOSRelease() (OSRelease, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		if os.IsNotExist(err) {
			return OSRelease{}, nil
		}
		return OSRelease{}, err
	}
	defer f.Close()
	return ParseOSRelease(f)
}

func ReadProcVersion() string {
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return ""
	}
	return string(b)
}

// UnameM runs uname once. Failure yields an empty string.
func UnameM() string {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
