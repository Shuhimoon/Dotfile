//go:build !windows

package main

// prepareConsole is a no-op outside Windows: Unix terminals wrap lazily and
// draw ambiguous-width characters one cell wide.
func prepareConsole() (restore func(), flags consoleFlags) {
	return func() {}, consoleFlags{ASCII: asciiOverride(false)}
}
