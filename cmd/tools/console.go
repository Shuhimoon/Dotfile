package main

import "os"

// consoleFlags are the Windows console workarounds passed to the TUI.
type consoleFlags struct {
	ASCII             bool
	ReserveLastColumn bool
}

// asciiOverride reads TOOLS_ASCII: "1" forces ASCII drawing, "0" turns it off.
func asciiOverride(def bool) bool {
	switch os.Getenv("TOOLS_ASCII") {
	case "1":
		return true
	case "0":
		return false
	}
	return def
}
