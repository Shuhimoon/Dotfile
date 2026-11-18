package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	dotfiles "github.com/shuhi/dotfiles"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/installlog"
	"github.com/shuhi/dotfiles/internal/platform"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/shellrc"
	"github.com/shuhi/dotfiles/internal/tui"
)

const usage = `用法:
  tools [--verbose] [--force]
  tools --version
  tools apply [--only id,id] [--pm apt|dnf|yum|pacman|zypper|apk|xbps|brew] [--dry-run] [--bashrc PATH] [--verbose] [--force]
  tools apply --config-only [--only zsh,starship,zellij] [--dry-run] [--verbose]
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin, stdout, stderr *os.File) int {
	verbose, force, args, bad := takeFrontFlags(args)
	if bad != "" {
		fmt.Fprintf(stderr, "未知參數 %s\n%s", bad, usage)
		return 2
	}
	if len(args) == 0 {
		if !isTTY(stdin) {
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runTUI(verbose, force, stdout)
	}
	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		fmt.Fprintln(stdout, versionString())
		return 0
	case "apply":
		return runApply(args[1:], verbose, force, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "未知參數 %s\n%s", args[0], usage)
		return 2
	}
}

// takeFrontFlags strips a leading --verbose and --force, each at most once, in either order.
func takeFrontFlags(args []string) (verbose, force bool, rest []string, bad string) {
	for len(args) > 0 {
		switch args[0] {
		case "--verbose":
			if verbose {
				return false, false, nil, "--verbose"
			}
			verbose = true
		case "--force":
			if force {
				return false, false, nil, "--force"
			}
			force = true
		default:
			return verbose, force, args, ""
		}
		args = args[1:]
	}
	return verbose, force, args, ""
}

func runTUI(verbose, force bool, stdout *os.File) int {
	fixOwner, _, err := sudoSetup(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	repo, repoErr := configRepo(home)
	fixOwner()
	uname := platform.UnameM()
	osr, _ := platform.LoadOSRelease()
	client := release.NewClients()
	restoreConsole, cflags := prepareConsole()
	defer restoreConsole()
	var m tea.Model = tui.New(platform.Info{})
	m = tui.SetDeps(m, tui.Deps{
		ASCII:             cflags.ASCII,
		ReserveLastColumn: cflags.ReserveLastColumn,
		FixOwner:          fixOwner,
		Styled:            isTTY(stdout) && os.Getenv("NO_COLOR") == "",
		Force:             force,
		Uname:             uname,
		Runner: install.Runner{
			Home:      home,
			XDG:       os.Getenv("XDG_CONFIG_HOME"),
			Bashrc:    filepath.Join(home, ".bashrc"),
			Zshrc:     filepath.Join(home, ".zshrc"),
			Repo:      repo,
			RepoErr:   errText(repoErr),
			GOOS:      runtime.GOOS,
			GOARCH:    runtime.GOARCH,
			Libc:      release.DetectLibc(osr.ID, release.LibcExists),
			Uname:     uname,
			Release:   client.Install,
			GoInstall: client.InstallGo,
			Rustup:    client.PrepareRustup,
			Deploy:    configcopy.Deploy,
			Rc:        shellrc.EnsureFile,
		},
		Log: func(manager string, results []install.ItemResult) error {
			return writeLog(home, manager, verbose, results)
		},
	})
	prog := tea.NewProgram(m, tea.WithAltScreen())
	final, err := prog.Run()
	fixOwner()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return tui.ExitCode(final)
}

func runApply(args []string, verbose, force bool, stdout, stderr *os.File) int {
	var only []string
	onlySet := false
	pmFlag := ""
	dry := false
	bashrc := ""
	configOnly := false
	pmSet, bashrcSet := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case a == "--verbose":
			verbose = true
		case a == "--force":
			force = true
		case a == "--dry-run":
			dry = true
		case a == "--config-only":
			configOnly = true
		case a == "--only":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "缺少 --only")
				return 2
			}
			i++
			only = splitCSV(args[i])
			onlySet = true
		case strings.HasPrefix(a, "--only="):
			only = splitCSV(strings.TrimPrefix(a, "--only="))
			onlySet = true
		case a == "--pm":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "缺少 --pm")
				return 2
			}
			i++
			pmFlag = args[i]
			pmSet = true
		case strings.HasPrefix(a, "--pm="):
			pmFlag = strings.TrimPrefix(a, "--pm=")
			pmSet = true
		case a == "--bashrc":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "缺少 --bashrc")
				return 2
			}
			i++
			bashrc = args[i]
			bashrcSet = true
		case strings.HasPrefix(a, "--bashrc="):
			bashrc = strings.TrimPrefix(a, "--bashrc=")
			bashrcSet = true
		default:
			fmt.Fprintf(stderr, "未知參數 %s\n%s", a, usage)
			return 2
		}
	}
	if configOnly {
		if pmSet || bashrcSet {
			fmt.Fprintln(stderr, "--config-only 不能搭配 --pm 或 --bashrc")
			return 2
		}
		return runConfigOnly(only, onlySet, dry, verbose, stdout, stderr)
	}
	var ids []catalog.ID
	if !onlySet {
		ids = catalog.DefaultIDs()
	} else {
		if len(only) == 0 {
			fmt.Fprintln(stderr, "尚未勾選任何項目")
			return 2
		}
		for _, id := range only {
			if !knownApplyID(catalog.ID(id)) {
				fmt.Fprintf(stderr, "未知的 ID %s\n", id)
				return 2
			}
			ids = append(ids, catalog.ID(id))
		}
	}
	if runtime.GOOS == "windows" {
		fmt.Fprintln(stderr, "Windows 不支援 apply：請直接執行 tools，用互動介面以 Scoop 安裝")
		return 2
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		fmt.Fprintf(stderr, "不支援的 GOOS=%s：只支援 linux、darwin 與 windows（互動介面）\n", runtime.GOOS)
		return 2
	}
	osr, err := platform.LoadOSRelease()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	look := func(bin string) bool {
		_, e := exec.LookPath(bin)
		return e == nil
	}
	var manager string
	if pmFlag != "" {
		if err := pm.CheckExplicit(pmFlag, look); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		manager = pmFlag
	} else {
		manager, err = pm.DetectManager(runtime.GOOS, osr, look)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	fixOwner, _, err := sudoSetup(stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if bashrc == "" {
		bashrc = filepath.Join(home, ".bashrc")
	}
	plans, err := install.ResolveAll(ids, manager)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	steps, err := install.BuildSteps(plans, os.Geteuid(), home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if dry {
		for _, st := range steps {
			fmt.Fprintln(stdout, st.Title)
			if len(st.Argv) > 0 {
				fmt.Fprintln(stdout, strings.Join(st.Argv, " "))
			}
		}
		return 0
	}
	repo, repoErr := configRepo(home)
	fixOwner()
	uname := platform.UnameM()
	client := release.NewClients()
	r := &install.Runner{
		Exec:      install.OSExecutor{},
		EUID:      os.Geteuid(),
		Force:     force,
		Home:      home,
		XDG:       os.Getenv("XDG_CONFIG_HOME"),
		Bashrc:    bashrc,
		Repo:      repo,
		RepoErr:   errText(repoErr),
		Manager:   manager,
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Libc:      release.DetectLibc(osr.ID, release.LibcExists),
		Uname:     uname,
		Release:   client.Install,
		GoInstall: client.InstallGo,
		Rustup:    client.PrepareRustup,
		Deploy:    configcopy.Deploy,
		Rc:        shellrc.EnsureFile,
		Log: func(results []install.ItemResult) error {
			return writeLog(home, manager, verbose, results)
		},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	results, code := r.RunSteps(ctx, steps)
	fixOwner()
	for _, res := range results {
		fmt.Fprintln(stdout, install.FormatResult(res))
	}
	if code == 2 {
		fmt.Fprintln(stderr, "更新失敗")
	}
	return code
}

// runConfigOnly deploys only the config files (and the zsh plugins when zsh is
// selected): no package manager, no update/package/release steps, no sudo.
// Without --only it covers every tool in configcopy.ToolIDs.
func runConfigOnly(only []string, onlySet, dry, verbose bool, stdout, stderr *os.File) int {
	var ids []catalog.ID
	if !onlySet {
		for _, id := range configcopy.ToolIDs {
			ids = append(ids, catalog.ID(id))
		}
	} else {
		if len(only) == 0 {
			fmt.Fprintln(stderr, "尚未勾選任何項目")
			return 2
		}
		for _, id := range only {
			if !configcopy.HasConfig(id) {
				fmt.Fprintf(stderr, "沒有設定檔的 ID %s（可用：%s）\n", id, strings.Join(configcopy.ToolIDs, ","))
				return 2
			}
			ids = append(ids, catalog.ID(id))
		}
	}
	// The zsh plugins are cloned with git. Only when git is missing does this
	// path look for a package manager and add the usual steps that install git.
	var git []install.Plan
	manager := ""
	if slices.Contains(ids, "zsh") && !lookPath("git") {
		osr, err := platform.LoadOSRelease()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		manager, err = pm.DetectManager(runtime.GOOS, osr, lookPath)
		if err != nil {
			fmt.Fprintln(stderr, "zsh 外掛需要 git，但無法安裝 git:", err)
			return 2
		}
		if git, err = install.ResolveAll([]catalog.ID{"git"}, manager); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	steps, err := install.BuildConfigSteps(ids, git, os.Geteuid())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if dry {
		for _, st := range steps {
			fmt.Fprintln(stdout, st.Title)
			if len(st.Argv) > 0 {
				fmt.Fprintln(stdout, strings.Join(st.Argv, " "))
			}
		}
		return 0
	}
	// sudoSetup never runs sudo; under "sudo tools" it points HOME back at the
	// invoking user and chowns what was written, as the other paths do.
	fixOwner, _, err := sudoSetup(stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	repo, repoErr := configRepo(home)
	fixOwner()
	seed := make([]install.ItemResult, 0, len(ids))
	for _, st := range steps {
		if st.Kind == install.KindConfig {
			seed = append(seed, install.ItemResult{ID: st.ItemID, Status: "ok", Note: "只部署設定"})
		}
	}
	r := &install.Runner{
		Exec:    install.OSExecutor{},
		EUID:    os.Geteuid(),
		Home:    home,
		XDG:     os.Getenv("XDG_CONFIG_HOME"),
		Repo:    repo,
		RepoErr: errText(repoErr),
		Deploy:  configcopy.Deploy,
		Seed:    seed,
		Manager: manager,
		Log: func(results []install.ItemResult) error {
			return writeLog(home, manager, verbose, results)
		},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	results, code := r.RunSteps(ctx, steps)
	fixOwner()
	for _, res := range results {
		fmt.Fprintln(stdout, install.FormatResult(res))
	}
	if code == 2 {
		fmt.Fprintln(stderr, "更新失敗：git 未安裝，zsh 外掛未下載")
	}
	return code
}

// lookPath reports whether bin is on PATH; tests replace it.
var lookPath = func(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func writeLog(home, manager string, verbose bool, results []install.ItemResult) error {
	items := make([]installlog.Item, 0, len(results))
	for _, res := range results {
		items = append(items, installlog.Item{
			ID:     string(res.ID),
			Method: string(res.Method),
			Argv:   res.Argv,
			Status: res.Status,
			Note:   res.Note,
			Err:    res.ErrText,
		})
	}
	rec := installlog.Record{
		Schema: 1,
		Time:   time.Now().UTC().Format(time.RFC3339),
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
		PM:     manager,
		Items:  items,
	}
	dir := filepath.Join(home, ".local", "share", "dotfiles-tools")
	return installlog.Write(dir, rec, verbose, os.Stderr)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// findRepo walks up from the working directory to a checkout of this repo
// (a go.mod declaring module github.com/shuhi/dotfiles). Any other go.mod
// stops nothing: the walk keeps going up.
func findRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRepoFrom(dir)
}

func findRepoFrom(dir string) (string, error) {
	for {
		if isDotfilesRepo(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("找不到 dotfiles repo")
		}
		dir = parent
	}
}

func isDotfilesRepo(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "module "+modulePath {
			return true
		}
	}
	return false
}

// configRepo is where config files are deployed from. Inside a checkout of
// this repo (development) the checkout itself is used. A released binary
// uses its embedded copy, extracted to
// ~/.local/share/dotfiles-tools/configs/<version>. "" means no configs
// (the config step then fails with the returned error, or with a generic
// reason when there is none).
func configRepo(home string) (string, error) {
	if repo, err := findRepo(); err == nil {
		return repo, nil
	}
	return extractConfigs(home, version)
}

// extractConfigs returns "" and no error without a home (nothing to extract
// into, as before); an extract failure is returned, not printed, so the TUI's
// alt screen cannot hide it.
func extractConfigs(home, ver string) (string, error) {
	if home == "" {
		return "", nil
	}
	dir := filepath.Join(home, ".local", "share", "dotfiles-tools", "configs", safeVersion(ver))
	if err := configcopy.Extract(dotfiles.Configs, dir); err != nil {
		return "", fmt.Errorf("設定檔解壓失敗: %w", err)
	}
	return dir, nil
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func safeVersion(v string) string {
	v = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_', r == '+':
			return r
		}
		return '_'
	}, v)
	if v == "" || v == "." || v == ".." {
		return "dev"
	}
	return v
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// knownApplyID is true for ids in the Linux/macOS catalog (apply never runs on Windows).
func knownApplyID(id catalog.ID) bool {
	for _, known := range catalog.IDs() {
		if known == id {
			return true
		}
	}
	return false
}
