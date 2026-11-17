package platform

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type OSRelease struct {
	ID, IDLike, Name, Version, VersionID, PrettyName string
}

func ParseOSRelease(r io.Reader) (OSRelease, error) {
	var out OSRelease
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = stripQuotes(strings.TrimSpace(val))
		switch key {
		case "ID":
			out.ID = val
		case "ID_LIKE":
			out.IDLike = val
		case "NAME":
			out.Name = val
		case "VERSION":
			out.Version = val
		case "VERSION_ID":
			out.VersionID = val
		case "PRETTY_NAME":
			out.PrettyName = val
		}
	}
	if err := sc.Err(); err != nil {
		return OSRelease{}, err
	}
	return out, nil
}

func stripQuotes(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

type OSInfo struct {
	GOOS       string
	Pretty     string
	WSL        bool
	SystemText string
}

func ClassifyOS(goos string, osr OSRelease, wsl bool) OSInfo {
	pretty := strings.TrimSpace(osr.PrettyName)
	if pretty == "" {
		pretty = strings.TrimSpace(strings.Join(strings.Fields(osr.Name+" "+osr.Version), " "))
	}
	if pretty == "" {
		if goos == "darwin" {
			pretty = "macOS"
		} else {
			pretty = goos
		}
	}
	bracket := fmt.Sprintf("[GOOS=%s]", goos)
	if wsl && !strings.Contains(strings.ToLower(pretty), "wsl") {
		bracket = "[GOOS=linux, WSL]"
	}
	return OSInfo{
		GOOS:       goos,
		Pretty:     pretty,
		WSL:        wsl,
		SystemText: pretty + " " + bracket,
	}
}
