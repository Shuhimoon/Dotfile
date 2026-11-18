//go:build windows

package tui

import (
	"errors"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// systemDNS lists the IPv4 DNS servers of the adapters that are up
// (Windows has no /etc/resolv.conf).
func systemDNS() string {
	size := uint32(15 * 1024)
	var buf []byte
	for i := 0; i < 3; i++ {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC,
			windows.GAA_FLAG_SKIP_UNICAST|windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST|windows.GAA_FLAG_SKIP_FRIENDLY_NAME,
			0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return identityFallback
		}
		buf = nil
	}
	if buf == nil {
		return identityFallback
	}
	var ips []net.IP
	for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
		if a.OperStatus != windows.IfOperStatusUp {
			continue
		}
		for d := a.FirstDnsServerAddress; d != nil; d = d.Next {
			if ip := d.Address.IP(); ip != nil {
				ips = append(ips, ip)
			}
		}
	}
	return dnsList(ips)
}
