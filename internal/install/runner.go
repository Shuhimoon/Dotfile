package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/configcopy"
	"github.com/shuhi/dotfiles/internal/pm"
	"github.com/shuhi/dotfiles/internal/release"
	"github.com/shuhi/dotfiles/internal/safepath"
	"github.com/shuhi/dotfiles/internal/shellrc"
)

type Runner struct {
	Exec      Executor
	EUID      int
	Home      string
	XDG       string
	Bashrc    string
	Zshrc     string
	Repo      string
	GoRoot    string
	DryRun    bool
	Manager   string
	GOOS      string
	GOARCH    string
	Libc      string
	Uname     string
	Exists    func(string) bool
	Release   func(ctx context.Context, repo, home string, q release.AssetQuery) error
	GoInstall func(ctx context.Context, q release.AssetQuery, euid int, dest string) error
	Rustup    func(ctx context.Context, q release.AssetQuery, dir string) ([]string, error)
	Deploy    func(src, dst string, policy configcopy.Policy, now time.Time, roots ...string) (configcopy.Result, error)
	// Rc writes managed blocks to one rc file; it is called for ~/.bashrc and ~/.zshrc.
	Rc  func(path, home string, sh shellrc.Shell, blocks []shellrc.BlockID) ([]shellrc.BlockID, error)
	Log func([]ItemResult) error
	Now func() time.Time
	// Force re-runs a package argv and xcode-select --install.
	// Preinstalled probes have no install argv and ignore it.
	Force bool
	// User is the account added to the docker group; nil uses SUDO_USER when
	// root, else the current user.
	User func() string
	// Progress is display only. An empty status means title is starting.
	Progress func(title, status string)
}

func (r *Runner) Run(ctx context.Context, plans []Plan) ([]ItemResult, int) {
	steps, err := BuildSteps(plans, r.EUID, r.Home)
	if err != nil {
		return nil, 2
	}
	return r.RunSteps(ctx, steps)
}

func (r *Runner) RunSteps(ctx context.Context, steps []Step) ([]ItemResult, int) {
	if r.DryRun {
		return nil, 0
	}
	var results []ItemResult
	var linkErr error
	for i := 0; i < len(steps); i++ {
		if err := ctx.Err(); err != nil {
			_ = r.writeLog(results)
			return results, 1
		}
		st := steps[i]
		r.note(st.Title, "")
		switch st.Kind {
		case KindUpdate:
			cls := r.execClass(ctx, st.Argv, st.Manager)
			if cls.Canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			if cls.ExecErr != nil || cls.ErrText != "" {
				return results, 2
			}
			r.note(st.Title, "ok")
		case KindConfig:
			updated, changed, canceled := r.doConfig(ctx, st, results)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			if changed {
				results = replaceResult(results, updated)
			}
			status := "skipped"
			if changed {
				status = updated.Status
				if status == "" {
					status = "failed"
				}
			} else if successStatus(updated.Status) {
				status = "ok"
			}
			r.note(st.Title, status)
		case KindLink:
			text, canceled := r.doLink(ctx, results)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			if text != "" {
				linkErr = errors.New(text)
			}
			status := "ok"
			if text != "" {
				status = "failed"
			}
			r.note(st.Title, status)
		case KindService:
			updated, status, canceled := r.doService(ctx, st, results)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			if status != "skipped" {
				results = replaceResult(results, updated)
			}
			r.note(st.Title, status)
		case KindZshPlugins:
			updated, status, canceled := r.doZshPlugins(ctx, results)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			if status != "skipped" {
				results = replaceResult(results, updated)
			}
			r.note(st.Title, status)
		case KindProbe:
			res := r.doProbe(st)
			results = append(results, res)
			r.note(st.Title, res.Status)
		case KindXcode:
			res, canceled := r.doXcode(ctx, st)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			results = append(results, res)
			r.note(st.Title, res.Status)
		case KindRelease:
			res, canceled := r.doRelease(ctx, st)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			results = append(results, res)
			r.note(st.Title, res.Status)
		case KindGo:
			res, canceled := r.doGo(ctx, st)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			results = append(results, res)
			r.note(st.Title, res.Status)
		case KindRustup:
			res, canceled := r.doRustup(ctx, st)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			results = append(results, res)
			r.note(st.Title, res.Status)
		default:
			res, canceled := r.doPackage(ctx, st)
			if canceled {
				_ = r.writeLog(results)
				return results, 1
			}
			results = append(results, res)
			title := st.Title
			if st.Fallback != nil && res.Method != st.Method && st.Fallback.Title != "" {
				title = st.Fallback.Title
			}
			r.note(title, res.Status)
		}
	}
	code := finalCode(results, linkErr)
	if err := r.writeLog(results); err != nil && code == 0 {
		code = 1
	}
	return results, code
}

func finalCode(results []ItemResult, linkErr error) int {
	if linkErr != nil {
		return 1
	}
	for _, res := range results {
		if res.Status == "failed" || res.Status == "pending-user" {
			return 1
		}
	}
	return 0
}

func (r *Runner) writeLog(results []ItemResult) error {
	if r == nil || r.DryRun || r.Log == nil {
		return nil
	}
	return r.Log(results)
}

func (r *Runner) note(title, status string) {
	if r.Progress != nil {
		r.Progress(title, status)
	}
}

func (r *Runner) execClass(ctx context.Context, argv []string, manager string) ExecClass {
	if len(argv) == 0 {
		return ExecClass{ExecErr: fmt.Errorf("空的 argv"), ErrText: "空的 argv"}
	}
	if r.Exec == nil {
		return ExecClass{ExecErr: fmt.Errorf("沒有 Executor"), ErrText: "沒有 Executor"}
	}
	out, code, err := r.Exec.Run(ctx, argv)
	return ClassExec(manager, out, code, err)
}

func (r *Runner) exists(path string) bool {
	if r.Exists != nil {
		return r.Exists(path)
	}
	_, err := os.Stat(path)
	return err == nil
}

func (r *Runner) query() release.AssetQuery {
	q := release.AssetQuery{GOOS: r.GOOS, GOARCH: r.GOARCH, Libc: r.Libc, UnameM: r.Uname}
	if q.GOOS == "" {
		q.GOOS = runtime.GOOS
	}
	if q.GOARCH == "" {
		q.GOARCH = runtime.GOARCH
	}
	return q
}

func (r *Runner) doPackage(ctx context.Context, st Step) (ItemResult, bool) {
	if !r.Force && AlreadySatisfied(st.Method, st.ProbeFiles, st.ProbeGroups, r.exists, false) {
		res := ItemResult{ID: st.ItemID, Method: st.Method, Status: "satisfied"}
		res.Note = WithFzfNote(st.ItemID, res.Status, "")
		// apt fd/bat still need the Debian command-name symlink; skip the rest of ConcludeExec.
		if st.Manager == "apt" && (st.ItemID == "fd" || st.ItemID == "bat") {
			ApplyAlias(r.Home, &res, r.exists)
			if res.Status == "failed" {
				res.Note = ""
			}
		}
		return res, false
	}
	cls := r.execClass(ctx, st.Argv, st.Manager)
	if cls.Canceled {
		return ItemResult{}, true
	}
	if cls.NotFound && st.Fallback != nil {
		return r.runFallback(ctx, st)
	}
	return ConcludeExec(st.ItemID, st.Manager, st.Method, st.Argv, cls, r.Home, r.exists), false
}

func (r *Runner) runFallback(ctx context.Context, st Step) (ItemResult, bool) {
	fb := *st.Fallback
	if fb.Title != "" {
		r.note(fb.Title, "")
	}
	res := ItemResult{ID: st.ItemID, Method: fb.Method}
	if fb.Local != nil {
		err := fb.Local(ctx)
		if errors.Is(err, context.Canceled) {
			return ItemResult{}, true
		}
		if err != nil {
			res.Status = "failed"
			res.ErrText = err.Error()
			return res, false
		}
		res.Status = "ok"
		if fb.Method == pm.MethodRelease {
			res.NeedsLocalBin = true
		}
		res.Note = WithFzfNote(res.ID, res.Status, "")
		return res, false
	}
	var canceled bool
	switch fb.Kind {
	case KindRelease:
		res, canceled = r.doRelease(ctx, fb)
	case KindGo:
		res, canceled = r.doGo(ctx, fb)
	case KindRustup:
		res, canceled = r.doRustup(ctx, fb)
	default:
		res, canceled = r.doPackage(ctx, fb)
	}
	if canceled {
		return ItemResult{}, true
	}
	res.ID = st.ItemID
	return res, false
}

func (r *Runner) doProbe(st Step) ItemResult {
	res := ItemResult{ID: st.ItemID, Method: pm.MethodPreinstalled}
	if AlreadySatisfied(st.Method, st.ProbeFiles, st.ProbeGroups, r.exists, false) {
		res.Status = "satisfied"
		res.Note = WithFzfNote(st.ItemID, res.Status, "")
		return res
	}
	res.Status = "failed"
	switch st.ItemID {
	case "make":
		res.ErrText = pm.MakeMissingMessage
	case "net-tools":
		res.ErrText = pm.NetToolsMissingMessage
	default:
		res.ErrText = "找不到預先安裝的檔案"
	}
	return res
}

func (r *Runner) doRelease(ctx context.Context, st Step) (ItemResult, bool) {
	res := ItemResult{ID: st.ItemID, Method: pm.MethodRelease}
	var err error
	if st.Local != nil {
		err = st.Local(ctx)
	} else if r.Release == nil {
		err = fmt.Errorf("未設定 release")
	} else {
		err = r.Release(ctx, st.ReleaseRepo, r.Home, r.query())
	}
	if errors.Is(err, context.Canceled) {
		return ItemResult{}, true
	}
	if err != nil {
		res.Status = "failed"
		res.ErrText = err.Error()
		return res, false
	}
	res.Status = "ok"
	res.NeedsLocalBin = true
	res.Note = WithFzfNote(st.ItemID, res.Status, "")
	return res, false
}

func (r *Runner) doGo(ctx context.Context, st Step) (ItemResult, bool) {
	res := ItemResult{ID: st.ItemID, Method: pm.MethodGoTarball}
	var err error
	if st.Local != nil {
		err = st.Local(ctx)
	} else if r.GoInstall == nil {
		err = fmt.Errorf("未設定 go tarball")
	} else {
		dest := r.GoRoot
		if dest == "" {
			dest = "/usr/local"
		}
		err = r.GoInstall(ctx, r.query(), r.EUID, dest)
	}
	if errors.Is(err, context.Canceled) {
		return ItemResult{}, true
	}
	if err != nil {
		res.Status = "failed"
		res.ErrText = err.Error()
		return res, false
	}
	res.Status = "ok"
	return res, false
}

func (r *Runner) doRustup(ctx context.Context, st Step) (ItemResult, bool) {
	res := ItemResult{ID: st.ItemID, Method: pm.MethodRustup}
	if st.Local != nil {
		err := st.Local(ctx)
		if errors.Is(err, context.Canceled) {
			return ItemResult{}, true
		}
		if err != nil {
			res.Status = "failed"
			res.ErrText = err.Error()
			return res, false
		}
		res.Status = "ok"
		return res, false
	}
	if r.Rustup == nil {
		res.Status = "failed"
		res.ErrText = "未設定 rustup"
		return res, false
	}
	dir, err := os.MkdirTemp("", "dotfiles-tools-rustup-")
	if err != nil {
		res.Status = "failed"
		res.ErrText = err.Error()
		return res, false
	}
	defer os.RemoveAll(dir)
	argv, err := r.Rustup(ctx, r.query(), dir)
	if errors.Is(err, context.Canceled) {
		return ItemResult{}, true
	}
	if err != nil {
		res.Status = "failed"
		res.ErrText = err.Error()
		return res, false
	}
	res.Argv = argv
	cls := r.execClass(ctx, argv, "")
	if cls.Canceled {
		return ItemResult{}, true
	}
	if cls.ExecErr != nil || cls.ErrText != "" {
		res.Status = "failed"
		res.ErrText = cls.ErrText
		return res, false
	}
	res.Status = "ok"
	return res, false
}

func (r *Runner) doXcode(ctx context.Context, st Step) (ItemResult, bool) {
	status, text, canceled := RunXcode(ctx, r.Exec, r.Force)
	if canceled {
		return ItemResult{}, true
	}
	return ItemResult{ID: st.ItemID, Method: pm.MethodXcodeCLT, Status: status, ErrText: text}, false
}

// RunXcode probes xcode-select, asks for the CLT dialog once, then probes again.
// force skips only the satisfied return; Canceled and ExecErr still do not install.
func RunXcode(ctx context.Context, exec Executor, force bool) (status, errText string, canceled bool) {
	if exec == nil {
		return "failed", "沒有 Executor", false
	}
	probe := func() ExecClass {
		out, code, err := exec.Run(ctx, []string{"xcode-select", "-p"})
		return ClassExec("", out, code, err)
	}
	cls := probe()
	if cls.Canceled {
		return "", "", true
	}
	if cls.ExecErr != nil {
		return "failed", cls.ErrText, false
	}
	if !force && XcodeSelectPresent(cls) {
		return "satisfied", "", false
	}
	out, code, err := exec.Run(ctx, []string{"xcode-select", "--install"})
	cls = ClassExec("", out, code, err)
	if cls.Canceled {
		return "", "", true
	}
	// --install often exits non-zero when the CLT is already present. That is not a start failure.
	if cls.ExecErr != nil {
		return "failed", cls.ErrText, false
	}
	cls = probe()
	if cls.Canceled {
		return "", "", true
	}
	if XcodeSelectPresent(cls) {
		return "satisfied", "", false
	}
	return "pending-user", pm.XcodePendingMessage, false
}

func (r *Runner) doConfig(ctx context.Context, st Step, results []ItemResult) (ItemResult, bool, bool) {
	cur, ok := findResult(results, st.ItemID)
	if !ok || !successStatus(cur.Status) {
		return cur, false, false
	}
	if st.Local != nil {
		err := st.Local(ctx)
		if errors.Is(err, context.Canceled) {
			return cur, false, true
		}
		if err != nil {
			cur.Status = "failed"
			cur.ErrText = err.Error()
			cur.Note = ""
			return cur, true, false
		}
		return cur, false, false
	}
	if r.Deploy == nil || r.Repo == "" || r.Home == "" {
		return cur, false, false
	}
	files, err := configcopy.FilesFor(string(st.ItemID), r.Repo, r.Home, r.XDG)
	if err != nil {
		cur.Status = "failed"
		cur.ErrText = err.Error()
		cur.Note = ""
		return cur, true, false
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return cur, false, true
		}
		roots := r.configRoots(f.Dst)
		if len(roots) == 0 {
			cur.Status = "failed"
			cur.ErrText = "拒絕目標 " + f.Dst
			cur.Note = ""
			return cur, true, false
		}
		if _, err := r.Deploy(f.Src, f.Dst, f.Policy, now, roots...); err != nil {
			if errors.Is(err, context.Canceled) {
				return cur, false, true
			}
			cur.Status = "failed"
			cur.ErrText = err.Error()
			cur.Note = ""
			return cur, true, false
		}
	}
	return cur, false, false
}

// configRoots are the roots dst may be written under: home, and an absolute XDG_CONFIG_HOME.
func (r *Runner) configRoots(dst string) []string {
	var roots []string
	if r.Home != "" && safepath.RejectEscapingSymlink(dst, r.Home) == nil {
		roots = append(roots, r.Home)
	}
	if r.XDG != "" && filepath.IsAbs(r.XDG) && safepath.RejectEscapingSymlink(dst, r.XDG) == nil {
		roots = append(roots, r.XDG)
	}
	return roots
}

// doLink writes the managed blocks to both ~/.bashrc and ~/.zshrc.
func (r *Runner) doLink(ctx context.Context, results []ItemResult) (string, bool) {
	if err := ctx.Err(); err != nil {
		return "", true
	}
	blocks := BlocksFor(results)
	if len(blocks) == 0 || r.Rc == nil {
		return "", false
	}
	var errs []string
	for _, sh := range shellrc.Shells {
		path := r.rcPath(sh)
		if _, err := r.Rc(path, r.Home, sh, blocks); err != nil {
			if errors.Is(err, context.Canceled) {
				return "", true
			}
			errs = append(errs, err.Error())
		}
	}
	return strings.Join(errs, "; "), false
}

func (r *Runner) rcPath(sh shellrc.Shell) string {
	switch {
	case sh == shellrc.Bash && r.Bashrc != "":
		return r.Bashrc
	case sh == shellrc.Zsh && r.Zshrc != "":
		return r.Zshrc
	}
	return shellrc.RcPath(r.Home, sh)
}

func findResult(results []ItemResult, id catalog.ID) (ItemResult, bool) {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].ID == id {
			return results[i], true
		}
	}
	return ItemResult{}, false
}

func replaceResult(results []ItemResult, updated ItemResult) []ItemResult {
	for i := range results {
		if results[i].ID == updated.ID {
			results[i] = updated
			return results
		}
	}
	return append(results, updated)
}
