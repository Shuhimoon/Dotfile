package configcopy

import (
	"bytes"
	"path/filepath"
)

// zjstatusRef is how the repo's zellij/config.kdl loads the shipped plugin.
const zjstatusRef = "file:~/.config/zellij/plugins/zjstatus.wasm"

// zellijConfig points config.kdl at the plugin deployed next to it
// (<dir of dst>/plugins/zjstatus.wasm), so an absolute $XDG_CONFIG_HOME works.
func zellijConfig(dst string, body []byte) []byte {
	wasm := "file:" + filepath.Join(filepath.Dir(dst), "plugins", "zjstatus.wasm")
	return bytes.ReplaceAll(body, []byte(zjstatusRef), []byte(wasm))
}
