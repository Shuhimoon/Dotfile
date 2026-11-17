package install

import "github.com/shuhi/dotfiles/internal/linkbash"

// HooksFor picks bash hooks from results. Order is fixed and ignores results order.
func HooksFor(results []ItemResult) []linkbash.HookID {
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
	var out []linkbash.HookID
	if local {
		out = append(out, linkbash.HookLocalBin)
	}
	if star {
		out = append(out, linkbash.HookStarship)
	}
	if fzf {
		out = append(out, linkbash.HookFzf)
	}
	if cargo {
		out = append(out, linkbash.HookCargoEnv)
	}
	if gopath {
		out = append(out, linkbash.HookGoPath)
	}
	return out
}
