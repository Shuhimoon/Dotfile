package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/shuhi/dotfiles/internal/install"
)

const (
	kindProgress = iota
	kindOutput
	kindDone
)

// bridgeMsg carries one event from the worker goroutine into Update.
// Commands never leave the alt screen: their output arrives here line by line.
type bridgeMsg struct {
	runID   int
	kind    int
	title   string
	status  string
	line    string
	cr      bool
	results []install.ItemResult
	code    int
}

func listen(ch <-chan bridgeMsg, runID int) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		msg.runID = runID
		return msg
	}
}

// sudoMode says how a "sudo ..." argv is run inside the TUI. The child never
// gets the terminal, so sudo must not try to prompt on /dev/tty.
type sudoMode int

const (
	// sudoCached runs "sudo -n": the credential cache was validated first.
	sudoCached sudoMode = iota
	// sudoStdin runs "sudo -S -p ''" and writes the password on stdin. It is
	// only used when sudoers keeps no cache (timestamp_timeout=0).
	sudoStdin
)

// streamExec runs a command with stdin closed (or the sudo password) and
// streams stdout+stderr line by line to the UI. It satisfies install.Executor.
type streamExec struct {
	ch   chan<- bridgeMsg
	mode sudoMode
	// pw is only set in sudoStdin mode. It is never logged or sent to the UI.
	pw []byte
}

// sudoArgv rewrites a leading "sudo" so it can never prompt on the terminal.
func sudoArgv(argv []string, mode sudoMode) []string {
	if len(argv) == 0 || argv[0] != "sudo" {
		return argv
	}
	rest := argv[1:]
	if mode == sudoStdin {
		return append([]string{"sudo", "-S", "-p", ""}, rest...)
	}
	return append([]string{"sudo", "-n"}, rest...)
}

func (e streamExec) Run(ctx context.Context, argv []string) (string, int, error) {
	if len(argv) == 0 {
		return "", 0, fmt.Errorf("空的 argv")
	}
	if err := ctx.Err(); err != nil {
		_, runErr := install.FinishWait(ctx, err)
		return "", 0, runErr
	}
	useStdin := e.mode == sudoStdin && argv[0] == "sudo"
	argv = sudoArgv(argv, e.mode)
	argv = scoopArgv(argv, exec.LookPath, statExists)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = install.CommandEnv(argv)
	// SIGTERM lets sudo relay the signal and package managers clean up.
	// Windows has no SIGTERM: kill the process there.
	cmd.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	if useStdin {
		cmd.Stdin = bytes.NewReader(append(append([]byte(nil), e.pw...), '\n'))
	}
	pr, pw := io.Pipe()
	var buf bytes.Buffer
	cmd.Stdout = pw
	cmd.Stderr = pw
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		splitLines(io.TeeReader(pr, &buf), func(line string, cr bool) {
			e.ch <- bridgeMsg{kind: kindOutput, line: line, cr: cr}
		})
	}()
	err := cmd.Start()
	if err != nil {
		pw.Close()
		wg.Wait()
		return "", 0, err
	}
	err = cmd.Wait()
	pw.Close()
	wg.Wait()
	code, runErr := install.FinishWait(ctx, err)
	return buf.String(), code, runErr
}

// splitLines calls emit for every line. cr is true when the line ended in a
// bare "\r" (a progress bar redraw), so the next line replaces it. "\r\n" is
// an ordinary line end.
func splitLines(r io.Reader, emit func(line string, cr bool)) {
	chunk := make([]byte, 4096)
	var cur []byte
	pendingCR := false
	for {
		n, err := r.Read(chunk)
		for _, b := range chunk[:n] {
			if pendingCR {
				pendingCR = false
				if b == '\n' {
					emit(cleanLine(cur), false)
					cur = cur[:0]
					continue
				}
				emit(cleanLine(cur), true)
				cur = cur[:0]
			}
			switch b {
			case '\n':
				emit(cleanLine(cur), false)
				cur = cur[:0]
			case '\r':
				pendingCR = true
			default:
				cur = append(cur, b)
			}
		}
		if err != nil {
			if pendingCR || len(cur) > 0 {
				emit(cleanLine(cur), false)
			}
			return
		}
	}
}

func cleanLine(b []byte) string {
	s := ansi.Strip(string(b))
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
