package tui

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

// sudoer is how the TUI talks to sudo without ever handing it the terminal.
//
// Flow: before a run whose steps contain "sudo ...", Cached checks
// "sudo -n true". If that fails, the TUI shows a masked password field and
// Validate runs "sudo -S -p ” -v" with the password on stdin. Every command
// of the run is then "sudo -n ..." (cached credential, refreshed by Refresh
// while the run lasts). Only when sudoers keeps no cache (timestamp_timeout=0)
// does the run fall back to "sudo -S -p ”" with the password on stdin; the
// password then lives in memory for that run only and is zeroed afterwards.
// The password is never logged, written to disk, or shown.
type sudoer interface {
	Cached(ctx context.Context) bool
	Validate(ctx context.Context, pw []byte) (cached bool, err error)
	Refresh(ctx context.Context)
}

type osSudo struct{}

func (osSudo) Cached(ctx context.Context) bool {
	return exec.CommandContext(ctx, "sudo", "-n", "true").Run() == nil
}

func (s osSudo) Validate(ctx context.Context, pw []byte) (bool, error) {
	in := append(append([]byte(nil), pw...), '\n')
	defer clear(in)
	cmd := exec.CommandContext(ctx, "sudo", "-S", "-p", "", "-v")
	cmd.Stdin = bytes.NewReader(in)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return false, sudoError(errb.String())
	}
	return s.Cached(ctx), nil
}

func (osSudo) Refresh(ctx context.Context) {
	_ = exec.CommandContext(ctx, "sudo", "-n", "-v").Run()
}

// sudoError keeps the last line sudo printed ("Sorry, try again.", "... is not
// in the sudoers file"). sudo never echoes the password.
func sudoError(stderr string) error {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return errors.New("sudo 驗證失敗")
	}
	return errors.New("sudo 驗證失敗：" + cleanLine([]byte(last)))
}

func (m model) sudo() sudoer {
	if m.deps.Sudo != nil {
		return m.deps.Sudo
	}
	return osSudo{}
}

func installNeedsSudo(steps []install.Step) bool {
	for _, st := range steps {
		if len(st.Argv) > 0 && st.Argv[0] == "sudo" {
			return true
		}
		if st.Fallback != nil && len(st.Fallback.Argv) > 0 && st.Fallback.Argv[0] == "sudo" {
			return true
		}
	}
	return false
}

func uninstallNeedsSudo(steps []uninstall.Step) bool {
	for _, st := range steps {
		if len(st.Argv) > 0 && st.Argv[0] == "sudo" {
			return true
		}
	}
	return false
}

type sudoCheckMsg struct {
	gen    int
	cached bool
}

type sudoValidMsg struct {
	gen    int
	cached bool
	err    error
	// pw comes back only when sudo keeps no cache and the run must feed it.
	pw []byte
}

// needSudo starts the sudo check for a prepared run, or the run itself.
func (m model) needSudo(need bool) (tea.Model, tea.Cmd) {
	if !need {
		return m.launch(sudoCached, nil)
	}
	m.phase = phaseSudo
	m.sudoGen++
	m.sudoBusy = true
	m.sudoNote = "檢查 sudo 權限…"
	gen := m.sudoGen
	s := m.sudo()
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sudoCheckMsg{gen: gen, cached: s.Cached(ctx)}
	}
}

func (m model) onSudoCheck(msg sudoCheckMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.sudoGen || m.phase != phaseSudo {
		return m, nil
	}
	m.sudoBusy = false
	m.sudoNote = ""
	if msg.cached {
		return m.launch(sudoCached, nil)
	}
	return m, nil
}

func (m model) onSudoValid(msg sudoValidMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.sudoGen || m.phase != phaseSudo {
		clear(msg.pw)
		return m, nil
	}
	m.sudoBusy = false
	if msg.err != nil {
		m.sudoNote = msg.err.Error()
		return m, nil
	}
	m.sudoNote = ""
	if msg.cached {
		return m.launch(sudoCached, nil)
	}
	return m.launch(sudoStdin, msg.pw)
}

func (m model) sudoKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		return m.leaveSudo(), nil
	}
	if m.sudoBusy {
		return m, nil
	}
	switch k.Type {
	case tea.KeyEnter:
		if len(m.pw) == 0 {
			return m, nil
		}
		pw := append([]byte(nil), m.pw...)
		m.clearPassword()
		m.sudoBusy = true
		m.sudoNote = "驗證中…"
		m.sudoGen++
		gen := m.sudoGen
		s := m.sudo()
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cached, err := s.Validate(ctx, pw)
			if err != nil || cached {
				clear(pw)
				pw = nil
			}
			return sudoValidMsg{gen: gen, cached: cached, err: err, pw: pw}
		}
	case tea.KeyBackspace:
		if len(m.pw) > 0 {
			_, size := utf8.DecodeLastRune(m.pw)
			m.pw = m.pw[:len(m.pw)-size]
		}
	case tea.KeyCtrlU:
		m.clearPassword()
	case tea.KeyRunes, tea.KeySpace:
		for _, r := range k.Runes {
			m.pw = utf8.AppendRune(m.pw, r)
		}
	}
	return m, nil
}

func (m *model) clearPassword() {
	clear(m.pw)
	m.pw = nil
}

func (m model) leaveSudo() model {
	m.clearPassword()
	m.sudoGen++
	m.sudoBusy = false
	m.sudoNote = ""
	m.phase = phaseBrowsing
	if m.mode == modeUninstall {
		m.phase = phaseUninstall
	}
	return m
}

func sudoBody(m model) []string {
	verb := "安裝"
	if m.mode == modeUninstall {
		verb = "移除"
	}
	lines := []string{
		verb + "需要 sudo 權限（套件管理器）。",
		"密碼只交給 sudo 驗證，不會記錄、顯示或寫入檔案。",
		"",
		"sudo 密碼：" + strings.Repeat("*", utf8.RuneCount(m.pw)),
	}
	if m.sudoNote != "" {
		lines = append(lines, "", m.sudoNote)
	}
	return lines
}
