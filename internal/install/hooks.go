package install

import "github.com/shuhi/dotfiles/internal/shellrc"

// BlocksFor picks the shell rc blocks from results. Order is fixed and ignores
// results order. The same blocks go to ~/.bashrc and ~/.zshrc.
func BlocksFor(results []ItemResult) []shellrc.BlockID {
	var local, star, fzf, cargo, gopath bool
	for _, res := range results {
		good := successStatus(res.Status)
		if good && res.NeedsLocalBin {
			local = true
		}
		if res.ID == "starship" && good {
			star = true
		}
		if res.ID == "fzf" && good {
			fzf = true
		}
		if res.ID == "rust" && res.Method == "rustup" && res.Status == "ok" {
			cargo = true
		}
		if res.ID == "go" && res.Method == "go-tarball" && res.Status == "ok" {
			gopath = true
		}
	}
	var out []shellrc.BlockID
	if local {
		out = append(out, shellrc.BlockLocalBin)
	}
	if star {
		out = append(out, shellrc.BlockStarship)
	}
	if fzf {
		out = append(out, shellrc.BlockFzf)
	}
	if cargo {
		out = append(out, shellrc.BlockRust)
	}
	if gopath {
		out = append(out, shellrc.BlockGo)
	}
	// The base block (PATH, cargo env, eza aliases, starship last) goes into both
	// rc files on every install run.
	return append(out, shellrc.BlockBase)
}
