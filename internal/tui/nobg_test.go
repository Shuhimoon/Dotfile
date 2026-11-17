package tui

import (
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
)

// TestNoScreenEmitsBackground: the user wants text colours only. No screen may
// emit a background SGR (48;...) or reverse video.
func TestNoScreenEmitsBackground(t *testing.T) {
	base := New(fedoraInfo())
	base.installed = map[catalog.ID]bool{"git": true, "jq": true, "python": true, "starship": true}
	base.homeUser, base.homeHost, base.homeIP, base.homeDNS = "shuhi", "box", "10.0.0.2", "1.1.1.1"
	base.steps = []install.Step{{Title: "install jq"}, {Title: "config starship"}}
	base.results = []install.ItemResult{{ID: "jq", Status: "ok"}}
	base.done = []doneLine{{Title: "install jq", Status: "ok"}}
	base.status = "狀態"

	screens := map[string]func(m model) model{
		"detecting": func(m model) model { m.phase = phaseDetecting; return m },
		"home":      func(m model) model { m.phase = phaseHome; return m },
		"home-sel2": func(m model) model { m.phase = phaseHome; m.homeIndex = 1; return m },
		"browse":    func(m model) model { m.phase = phaseBrowsing; return m },
		"failed":    func(m model) model { m.phase = phaseFailed; return m },
		"ask":       func(m model) model { m.phase = phaseBrowsing; m.askName = "git"; return m },
		"confirm":   func(m model) model { m.phase = phaseConfirm; return m },
		"installing": func(m model) model {
			m.phase = phaseInstalling
			m.curTitle = "config starship"
			m.running = true
			return m
		},
		"report": func(m model) model { m.phase = phaseReport; return m },
		"uninstall": func(m model) model {
			m.phase = phaseUninstall
			m.mode = modeUninstall
			m.uselected = map[catalog.ID]bool{"jq": true}
			return m
		},
		"uninstall-cursor": func(m model) model { m.phase = phaseUninstall; m.mode = modeUninstall; m.ucursor = 1; return m },
		"python-warn":      func(m model) model { m.phase = phasePythonWarn; m.mode = modeUninstall; return m },
		"uninstall-confirm": func(m model) model {
			m.phase = phaseConfirm
			m.mode = modeUninstall
			return m
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
		}
	}
}
