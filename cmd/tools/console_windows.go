//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// prepareConsole turns on DISABLE_NEWLINE_AUTO_RETURN so writing the last
// column does not move the cursor (Bubble Tea only enables VT processing),
// and asks the TUI to leave the last column empty and draw ASCII box lines
// in the classic console host. Windows Terminal (WT_SESSION set) draws box
// characters one cell wide, so it keeps them.
func prepareConsole() (restore func(), flags consoleFlags) {
	restore = func() {}
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err == nil {
		want := mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
		if want != mode && windows.SetConsoleMode(h, want) == nil {
			restore = func() { _ = windows.SetConsoleMode(h, mode) }
		}
	}
	flags = consoleFlags{
		ASCII:             asciiOverride(os.Getenv("WT_SESSION") == ""),
		ReserveLastColumn: true,
	}
	return restore, flags
}
