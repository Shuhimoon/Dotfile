package tui

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

// TestNoScreenEmitsBackground: the user wants text colours only. No screen,
// the progress page and the sudo page included, may emit a background SGR
// (48;...) or reverse video.
func TestNoScreenEmitsBackground(t *testing.T) {
	base := New(fedoraInfo())
	base.installed = map[catalog.ID]bool{"git": true, "jq": true, "python": true, "starship": true}
	base.homeUser, base.homeHost, base.homeIP, base.homeDNS = "shuhi", "box", "10.0.0.2", "1.1.1.1"
	base.steps = []install.Step{{Title: "install jq"}, {Title: "config starship"}, {Title: "install fzf"}}
	base.results = []install.ItemResult{{ID: "jq", Status: "ok"}, {ID: "fzf", Status: "failed", ErrText: "boom"}}
	base.status = "狀態"

	running := func(m model) model {
		m.phase = phaseInstalling
		m.stepState = make([]string, len(m.steps))
		m.applyProgress("install jq", "")
		m.applyProgress("install jq", "ok")
		m.applyProgress("config starship", "")
		for _, l := range []string{"Downloading starship…", "50%", "done"} {
			m.appendOutput(l, false)
		}
		return m
	}
	screens := map[string]func(m model) model{
		"detecting":  func(m model) model { m.phase = phaseDetecting; return m },
		"home":       func(m model) model { m.phase = phaseHome; return m },
		"home-sel2":  func(m model) model { m.phase = phaseHome; m.homeIndex = 1; return m },
		"browse":     func(m model) model { m.phase = phaseBrowsing; return m },
		"browse-sel": func(m model) model { m.phase = phaseBrowsing; m.selected["git"] = true; return m },
		"failed":     func(m model) model { m.phase = phaseFailed; return m },
		"sudo":       func(m model) model { m.phase = phaseSudo; m.pw = []byte("abc"); return m },
		"sudo-err": func(m model) model {
			m.phase = phaseSudo
			m.mode = modeUninstall
			m.sudoNote = "sudo 驗證失敗：Sorry, try again."
			return m
		},
		"installing": running,
		"canceling":  func(m model) model { m = running(m); m.canceled = true; return m },
		"report":     func(m model) model { m = running(m); m.finishRun(m.results, 1); return m },
		"dry":        func(m model) model { m.dryRun = true; return m.dryReport() },
		"uninstall": func(m model) model {
			m.phase = phaseUninstall
			m.mode = modeUninstall
			m.uselected = map[catalog.ID]bool{"jq": true}
			return m
		},
		"uninstall-cursor": func(m model) model { m.phase = phaseUninstall; m.mode = modeUninstall; m.ucursor = 1; return m },
		"uninstall-empty":  func(m model) model { m.installed = nil; return m.enterUninstallModel() },
		"uninstall-progress": func(m model) model {
			m.mode = modeUninstall
			m.warnings = []string{uninstall.PythonWarning}
			return running(m)
		},
	}
	for name, mk := range screens {
		for _, size := range [][2]int{{80, 40}, {120, 50}, {30, 12}, {4, 6}} {
			m := mk(base)
			m.width, m.height = size[0], size[1]
			view := renderStyled(m, testRenderer())
			if strings.Contains(view, "48;2;") || strings.Contains(view, "48;5;") || hasReverse(view) {
				t.Errorf("%s %dx%d emits a background:\n%q", name, size[0], size[1], view)
			}
			if size[0] >= 80 && !strings.Contains(view, "\x1b[") {
				t.Errorf("%s %dx%d lost its text colours", name, size[0], size[1])
			}
			if strings.HasSuffix(view, "\n") {
				t.Errorf("%s %dx%d ends in a newline: Bubble Tea would cut the top line", name, size[0], size[1])
			}
			if got := len(frameLines(view)); got != size[1] {
				t.Errorf("%s %dx%d has %d lines", name, size[0], size[1], got)
			}
		}
	}
}

func (m model) enterUninstallModel() model {
	next, _ := m.enterUninstall()
	return next.(model)
}
