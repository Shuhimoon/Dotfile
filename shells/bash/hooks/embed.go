package hooks

import _ "embed"

//go:embed local-bin.bash
var LocalBin []byte

//go:embed starship.bash
var Starship []byte

//go:embed fzf.bash
var Fzf []byte

//go:embed cargo-env.bash
var CargoEnv []byte

//go:embed go-path.bash
var GoPath []byte

func Bytes(id string) ([]byte, bool) {
	switch id {
	case "local-bin":
		return LocalBin, true
	case "starship":
		return Starship, true
	case "fzf":
		return Fzf, true
	case "cargo-env":
		return CargoEnv, true
	case "go-path":
		return GoPath, true
	default:
		return nil, false
	}
}
