package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/install"
)

const (
	kindAsk = iota
	kindProgress
	kindDone
)

type execReply struct {
	out  string
	code int
	err  error
}

// bridgeMsg is the only channel between Runner and Bubble Tea.
// tea.ExecProcess has to be returned from Update, so Executor.Run cannot call it.
type bridgeMsg struct {
	runID   int
	kind    int
	argv    []string
	ctx     context.Context
	reply   chan execReply
	title   string
	status  string
	results []install.ItemResult
	code    int
}

type listenAgainMsg struct {
	runID int
}

type teaExec struct {
	ch chan<- bridgeMsg
}

func (e teaExec) Run(ctx context.Context, argv []string) (string, int, error) {
	if len(argv) == 0 {
		return "", 0, fmt.Errorf("空的 argv")
	}
	if err := ctx.Err(); err != nil {
		_, runErr := install.FinishWait(ctx, err)
		return "", 0, runErr
	}
	reply := make(chan execReply, 1)
	select {
	case <-ctx.Done():
		return "", 0, context.Canceled
	case e.ch <- bridgeMsg{kind: kindAsk, argv: argv, ctx: ctx, reply: reply}:
	}
	res := <-reply
	return res.out, res.code, res.err
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

func (m model) onBridge(msg bridgeMsg) (tea.Model, tea.Cmd) {
	if msg.runID != m.runID || m.phase != phaseInstalling {
		if msg.kind == kindAsk && msg.reply != nil {
			msg.reply <- execReply{err: context.Canceled}
		}
		return m, nil
	}
	switch msg.kind {
	case kindAsk:
		if msg.ctx != nil && msg.ctx.Err() != nil {
			msg.reply <- execReply{err: context.Canceled}
			return m, listen(m.events, m.runID)
		}
		return m, m.execProcess(msg)
	case kindProgress:
		if msg.status == "" {
			m.curTitle = msg.title
			m.running = true
		} else {
			m.done = append(m.done, doneLine{Title: msg.title, Status: msg.status})
			m.running = false
			m.curTitle = msg.title
		}
		return m, listen(m.events, m.runID)
	case kindDone:
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.results = msg.results
		m.exitCode = msg.code
		m.running = false
		if msg.code == 2 && m.status == "" {
			m.status = "更新失敗"
		}
		m.phase = phaseReport
		return m, refreshInstalled(m.info.Manager, m.runID)
	default:
		return m, listen(m.events, m.runID)
	}
}

func (m model) execProcess(msg bridgeMsg) tea.Cmd {
	cmd := exec.CommandContext(msg.ctx, msg.argv[0], msg.argv[1:]...)
	buf := &bytes.Buffer{}
	cmd.Stdout = io.MultiWriter(os.Stdout, buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, buf)
	runID := m.runID
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		code, runErr := install.FinishWait(msg.ctx, err)
		msg.reply <- execReply{out: buf.String(), code: code, err: runErr}
		return listenAgainMsg{runID: runID}
	})
}
