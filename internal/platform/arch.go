package platform

func ClassifyArch(goarch, unameM string) ArchInfo {
	info := ArchInfo{GOARCH: goarch, UnameM: unameM}
	switch goarch {
	case "amd64":
		info.Family = "x86_64"
	case "386":
		info.Family = "i386"
	case "arm", "arm64":
		info.Family = "ARM"
	case "":
		info.Family = "unknown"
	default:
		info.Family = goarch
	}
	return info
}

type ArchInfo struct {
	Family string
	GOARCH string
	UnameM string
}
