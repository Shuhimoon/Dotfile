package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/linkbash"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	bashrc := ""
	hooksFlag := ""
	hooksSet := false
	require := true
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case a == "--bashrc":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "缺少 --bashrc 的路徑")
				return 2
			}
			i++
			bashrc = args[i]
		case strings.HasPrefix(a, "--bashrc="):
			bashrc = strings.TrimPrefix(a, "--bashrc=")
		case a == "--hooks":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "缺少 --hooks")
				return 2
			}
			i++
			hooksFlag = args[i]
			hooksSet = true
		case strings.HasPrefix(a, "--hooks="):
			hooksFlag = strings.TrimPrefix(a, "--hooks=")
			hooksSet = true
		case a == "--require-cmd":
			if i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
				require = args[i+1] == "true"
				i++
			} else {
				require = true
			}
		case strings.HasPrefix(a, "--require-cmd="):
			require = strings.TrimPrefix(a, "--require-cmd=") != "false"
		default:
			fmt.Fprintf(stderr, "未知參數 %s\n%s", a, usage)
			return 2
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if bashrc == "" {
		bashrc = filepath.Join(home, ".bashrc")
	}
	var ids []linkbash.HookID
	if !hooksSet {
		ids = []linkbash.HookID{linkbash.HookLocalBin, linkbash.HookStarship, linkbash.HookFzf}
	} else {
		for _, part := range strings.Split(hooksFlag, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			ids = append(ids, linkbash.HookID(part))
		}
	}
	ids = filterRequire(ids, require, home)
	rep, err := linkbash.Apply(bashrc, ids)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, id := range rep.Added {
		fmt.Fprintf(stdout, "added %s\n", id)
	}
	for _, id := range rep.Skipped {
		fmt.Fprintf(stdout, "skipped %s\n", id)
	}
	return 0
}

func filterRequire(ids []linkbash.HookID, require bool, home string) []linkbash.HookID {
	if !require {
		return ids
	}
	look := func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
	fi, err := os.Stat(filepath.Join(home, ".local", "bin"))
	localExists := err == nil && fi.IsDir()
	starshipKept := false
	fzfKept := false
	wantLocal := false
	var keep []linkbash.HookID
	for _, id := range ids {
		switch id {
		case linkbash.HookStarship:
			if look("starship") {
				keep = append(keep, id)
				starshipKept = true
			}
		case linkbash.HookFzf:
			if look("fzf") {
				keep = append(keep, id)
				fzfKept = true
			}
		case linkbash.HookLocalBin:
			wantLocal = true
		default:
			keep = append(keep, id)
		}
	}
	if wantLocal && (localExists || starshipKept || fzfKept) {
		keep = append(keep, linkbash.HookLocalBin)
	}
	return keep
}

const usage = `用法:
  link-bash [--bashrc PATH] [--hooks local-bin,starship,fzf,cargo-env,go-path] [--require-cmd=true]
`
