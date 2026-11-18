//go:build !windows

package tui

import "os"

func systemDNS() string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return identityFallback
	}
	return parseResolv(string(data))
}
