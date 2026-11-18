package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/pm"
)

type Executor interface {
	// err is only a start failure or a canceled context.
	// A process that started and exited non-zero returns err == nil and that exitCode.
	Run(ctx context.Context, argv []string) (combined string, exitCode int, err error)
}

type ExecClass struct {
	ExitCode int
	Canceled bool
	ExecErr  error
	NotFound bool
	ErrText  string
}

// ClassExec normalizes a wait error and, when the process actually exited, classifies it.
// Pass the Executor exitCode when err is already nil. A raw *exec.ExitError may be passed
// as waitErr with exitCode 0; it is converted to a non-zero code and a nil exec error.
func ClassExec(manager, output string, exitCode int, waitErr error) ExecClass {
	if waitErr != nil {
		code, err := InterpretWait(waitErr)
		if errors.Is(err, context.Canceled) {
			return ExecClass{Canceled: true, ExecErr: err, ExitCode: code}
		}
		if err != nil {
			return ExecClass{ExecErr: err, ErrText: err.Error()}
		}
		exitCode = code
	}
	// 130 is the status of a child that caught SIGINT. It cancels the install.
	if exitCode == 130 {
		return ExecClass{Canceled: true, ExecErr: context.Canceled, ExitCode: 130}
	}
	c := ExecClass{ExitCode: exitCode}
	if exitCode == 0 {
		return c
	}
	cerr := pm.ClassifyFailure(manager, exitCode, output)
	if cerr == nil {
		return c
	}
	c.ErrText = cerr.Error()
	var nf *pm.NotFoundError
	if errors.As(cerr, &nf) {
		c.NotFound = true
	}
	return c
}

// FinishWait maps a started command onto the Executor contract.
// Context cancel and exit 130 both return context.Canceled, even when the
// child also produced a status. Other non-zero exits are err == nil.
func FinishWait(ctx context.Context, waitErr error) (int, error) {
	if ctx != nil && errors.Is(ctx.Err(), context.Canceled) {
		return 0, context.Canceled
	}
	code, err := InterpretWait(waitErr)
	if err == nil && code == 130 {
		return 130, context.Canceled
	}
	return code, err
}

// InterpretWait turns a process wait error into the Executor contract.
func InterpretWait(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	if errors.Is(err, context.Canceled) {
		return 0, context.Canceled
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGINT {
			return 0, context.Canceled
		}
		return ee.ExitCode(), nil
	}
	return 0, err
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, argv []string) (string, int, error) {
	if len(argv) == 0 {
		return "", 0, fmt.Errorf("空的 argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = CommandEnv(argv)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	err := cmd.Run()
	code, runErr := FinishWait(ctx, err)
	return buf.String(), code, runErr
}

func Decorate(id catalog.ID, manager string, method pm.Method) (status, note string) {
	if id == "trash" && manager == "brew" && method == pm.MethodPackage {
		return "shadowed", pm.TrashShadowNote
	}
	status = "ok"
	if method == pm.MethodPackage {
		note = pm.KegOnlyNote(id, manager, method)
	}
	return status, note
}

func WithFzfNote(id catalog.ID, status, note string) string {
	if id != "fzf" {
		return note
	}
	switch status {
	case "ok", "satisfied", "shadowed":
	default:
		return note
	}
	const extra = "fzf 0.48.0 以上"
	if note == "" {
		return extra
	}
	if strings.Contains(note, extra) {
		return note
	}
	return note + "  " + extra
}

func ConcludeExec(id catalog.ID, manager string, method pm.Method, argv []string, cls ExecClass, home string, exists func(string) bool) ItemResult {
	res := ItemResult{ID: id, Method: method, Argv: append([]string(nil), argv...)}
	if cls.ExecErr != nil || cls.ErrText != "" {
		res.Status = "failed"
		res.ErrText = cls.ErrText
		if cls.NotFound && id == "whois" && manager == "yum" {
			res.ErrText = pm.EPELMessage
		}
		return res
	}
	res.Status, res.Note = Decorate(id, manager, method)
	if method == pm.MethodRelease {
		res.NeedsLocalBin = true
	}
	ApplyAlias(home, &res, exists)
	if res.Status == "failed" {
		res.Note = ""
		return res
	}
	res.Note = WithFzfNote(id, res.Status, res.Note)
	return res
}

func ProbeHit(files []string, exists func(string) bool) bool {
	if exists == nil {
		return false
	}
	for _, f := range files {
		if f != "" && exists(f) {
			return true
		}
	}
	return false
}

// xcodeOK is XcodeSelectPresent of a ClassExec result. Callers return on
// Canceled and ExecErr before using a false value as "not installed".
// Non-empty groups are AND of ProbeHit and ignore the flat probe list.
func AlreadySatisfied(method pm.Method, probes []string, groups [][]string, exists func(string) bool, xcodeOK bool) bool {
	switch method {
	case pm.MethodPackage, pm.MethodPreinstalled:
		if len(groups) > 0 {
			return probeGroups(groups, exists)
		}
		return len(probes) > 0 && ProbeHit(probes, exists)
	case pm.MethodXcodeCLT:
		return xcodeOK
	default:
		return false
	}
}

func probeGroups(groups [][]string, exists func(string) bool) bool {
	if exists == nil || len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if !ProbeHit(g, exists) {
			return false
		}
	}
	return true
}

// XcodeSelectPresent reports whether ClassExec of ["xcode-select", "-p"]
// means the CLT is present. Exit 0 leaves ErrText empty even when stdout is
// the developer directory; do not pass that stdout as ErrText.
// Canceled and ExecErr are not present and must not fall through to --install.
func XcodeSelectPresent(cls ExecClass) bool {
	return !cls.Canceled && cls.ExecErr == nil && cls.ErrText == "" && cls.ExitCode == 0
}

func successStatus(status string) bool {
	return status == "ok" || status == "satisfied" || status == "shadowed"
}
