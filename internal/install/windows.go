package install

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

const (
	// KindScoopBootstrap installs Scoop itself when it is missing.
	KindScoopBootstrap Kind = "scoop-bootstrap"
	// KindScoopBucket adds a Scoop bucket (extras for wezterm).
	KindScoopBucket Kind = "scoop-bucket"
)

// ScoopStepID is the result id of the bootstrap step.
const ScoopStepID catalog.ID = "scoop"

// WinEnv is what the Windows step builder needs to know about the machine.
type WinEnv struct {
	// ScoopRoot is %SCOOP% or ~\scoop.
	ScoopRoot string
	// Look is exec.LookPath; nil means "not on PATH".
	Look func(bin string) (string, bool)
}

func (e WinEnv) look(bin string) []string {
	if e.Look == nil {
		return nil
	}
	if p, ok := e.Look(bin); ok && p != "" {
		return []string{p}
	}
	return nil
}

// BuildWindowsSteps is the Windows install plan: bootstrap Scoop when needed,
// add the extras bucket when an item lives there, then one scoop install per
// item in catalog order. There is no update step, no sudo, no config copy and
// no shell rc step on Windows.
func BuildWindowsSteps(ids []catalog.ID, env WinEnv) ([]Step, error) {
	want := map[catalog.ID]bool{}
	for _, id := range ids {
		if _, ok := pm.ScoopFor(id); !ok {
			return nil, fmt.Errorf("Windows 不支援 %s", id)
		}
		want[id] = true
	}
	if len(want) == 0 {
		return nil, nil
	}
	steps := []Step{{
		ItemID:     ScoopStepID,
		Title:      "scoop",
		Argv:       pm.ScoopBootstrapArgv(),
		Kind:       KindScoopBootstrap,
		Manager:    pm.Scoop,
		Method:     pm.MethodPackage,
		ProbeFiles: append(pm.ScoopCommands(env.ScoopRoot), env.look("scoop")...),
	}}
	buckets := map[string]bool{}
	for _, id := range catalog.IDsFor("windows") {
		if !want[id] {
			continue
		}
		it, _ := pm.ScoopFor(id)
		if it.Bucket != "" && !buckets[it.Bucket] {
			buckets[it.Bucket] = true
			argv := pm.ScoopBucketArgv(it.Bucket)
			if err := pm.ValidateArgv(argv); err != nil {
				return nil, err
			}
			steps = append(steps, Step{
				ItemID:     id,
				Title:      strings.Join(argv, " "),
				Argv:       argv,
				Kind:       KindScoopBucket,
				Manager:    pm.Scoop,
				Method:     pm.MethodPackage,
				ProbeFiles: []string{filepath.Join(env.ScoopRoot, "buckets", it.Bucket)},
			})
		}
		argv := it.InstallArgv()
		if err := pm.ValidateArgv(argv); err != nil {
			return nil, err
		}
		steps = append(steps, Step{
			ItemID:     id,
			Title:      string(id),
			Argv:       argv,
			Kind:       KindPackage,
			Manager:    pm.Scoop,
			Method:     pm.MethodPackage,
			ProbeFiles: append([]string{pm.ScoopShim(env.ScoopRoot, it.Binary)}, env.look(it.Binary)...),
		})
	}
	return steps, nil
}

// WindowsInstalled marks the Windows items found as a Scoop shim or on PATH.
func WindowsInstalled(env WinEnv, exists func(string) bool) map[catalog.ID]bool {
	out := map[catalog.ID]bool{}
	for _, id := range catalog.IDsFor("windows") {
		it, _ := pm.ScoopFor(id)
		if (exists != nil && exists(pm.ScoopShim(env.ScoopRoot, it.Binary))) || len(env.look(it.Binary)) > 0 {
			out[id] = true
		}
	}
	return out
}

// doScoopBootstrap runs the official installer unless Scoop is already there.
// A failure stops the run: nothing else can install without Scoop.
func (r *Runner) doScoopBootstrap(ctx context.Context, st Step) (res ItemResult, stop, canceled bool) {
	res = ItemResult{ID: ScoopStepID, Method: pm.MethodPackage}
	if ProbeHit(st.ProbeFiles, r.exists) {
		res.Status = "satisfied"
		return res, false, false
	}
	cls := r.execClass(ctx, st.Argv, st.Manager)
	if cls.Canceled {
		return res, true, true
	}
	if cls.ExecErr != nil || cls.ErrText != "" {
		res.Status = "failed"
		res.ErrText = "安裝 Scoop 失敗：" + cls.ErrText
		return res, true, false
	}
	res.Status = "ok"
	res.Note = "已安裝 Scoop（重新開啟終端機後 scoop 才會在 PATH 上）"
	return res, false, false
}

// doScoopBucket adds a bucket. "already exists" counts as success.
func (r *Runner) doScoopBucket(ctx context.Context, st Step) (status string, canceled bool) {
	if ProbeHit(st.ProbeFiles, r.exists) {
		return "satisfied", false
	}
	if r.Exec == nil {
		return "failed", false
	}
	out, code, err := r.Exec.Run(ctx, st.Argv)
	cls := ClassExec(st.Manager, out, code, err)
	if cls.Canceled {
		return "", true
	}
	if cls.ExecErr == nil && (cls.ExitCode == 0 || strings.Contains(out, "already exists")) {
		return "ok", false
	}
	return "failed", false
}
