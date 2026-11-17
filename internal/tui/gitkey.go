package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shuhi/dotfiles/internal/catalog"
	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/sshkey"
)

// GitHub SSH key setup. After a real install run in which git ended up
// installed (ok, satisfied or shadowed), the report's any-key goes here
// instead of home. The user picks a private key from the key directory, or
// pastes a new one and names it; ~/.ssh/config then gets the managed
// "# >>> dotfiles:github >>>" block. esc skips. Key text is kept only in
// memory while being pasted, never shown (only its header line and size),
// and zeroed when saved or left.

type gitStage int

const (
	gitPick gitStage = iota
	gitAdd
	gitDone
)

const gitAddNew = "＋ 貼上新的私鑰…"

func (m model) goos() string {
	if m.deps.GOOS != "" {
		return m.deps.GOOS
	}
	return runtime.GOOS
}

func (m model) sshLayout() sshkey.Layout {
	return sshkey.LayoutFor(m.goos(), m.home())
}

// gitReady says the run just finished put git in place.
func gitReady(results []install.ItemResult) bool {
	for _, r := range results {
		if r.ID == catalog.ID("git") {
			switch r.Status {
			case "ok", "satisfied", "shadowed":
				return true
			}
		}
	}
	return false
}

func (m model) enterGitKey() (tea.Model, tea.Cmd) {
	m.gitPending = false
	m.phase = phaseGitKey
	m.gitNote = ""
	m.gitDoneLines = nil
	keys, err := sshkey.ListKeys(m.sshLayout().KeyDir)
	if err != nil {
		m.gitNote = "讀不到鑰匙目錄：" + err.Error()
	}
	m.gitKeys = keys
	m.gitCursor = 0
	if len(keys) == 0 {
		m.gitStage = gitAdd
		m.gitFocus = 0
		return m, nil
	}
	m.gitStage = gitPick
	return m, nil
}

func (m *model) clearGitInput() {
	clear(m.gitBuf)
	m.gitBuf = nil
	m.gitName = nil
}

func (m model) leaveGitKey() (tea.Model, tea.Cmd) {
	m.clearGitInput()
	m.gitKeys = nil
	m.gitNote = ""
	m.gitDoneLines = nil
	return m.goHome()
}

func (m model) gitKeyKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.gitStage {
	case gitDone:
		return m.leaveGitKey()
	case gitPick:
		switch k.String() {
		case "esc", "q", "ctrl+c":
			return m.leaveGitKey()
		case "up", "k":
			if m.gitCursor > 0 {
				m.gitCursor--
			}
		case "down", "j":
			if m.gitCursor < len(m.gitKeys) {
				m.gitCursor++
			}
		case "enter":
			if m.gitCursor == len(m.gitKeys) {
				m.gitStage = gitAdd
				m.gitFocus = 0
				m.gitNote = ""
				return m, nil
			}
			return m.applyGitKey(m.gitKeys[m.gitCursor], "")
		}
		return m, nil
	}
	// gitAdd
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.clearGitInput()
		m.gitNote = ""
		if len(m.gitKeys) > 0 {
			m.gitStage = gitPick
			return m, nil
		}
		return m.leaveGitKey()
	case tea.KeyTab, tea.KeyShiftTab:
		m.gitFocus = 1 - m.gitFocus
		return m, nil
	case tea.KeyCtrlS:
		return m.saveGitKey()
	case tea.KeyCtrlU:
		if m.gitFocus == 0 {
			clear(m.gitBuf)
			m.gitBuf = nil
		} else {
			m.gitName = nil
		}
		return m, nil
	}
	if m.gitFocus == 0 {
		switch k.Type {
		case tea.KeyEnter:
			// Without bracketed paste each pasted newline arrives as enter.
			m.gitBuf = append(m.gitBuf, '\n')
		case tea.KeyBackspace:
			if len(m.gitBuf) > 0 {
				_, size := utf8.DecodeLastRune(m.gitBuf)
				m.gitBuf = m.gitBuf[:len(m.gitBuf)-size]
			}
		case tea.KeyRunes, tea.KeySpace:
			for _, r := range k.Runes {
				m.gitBuf = utf8.AppendRune(m.gitBuf, r)
			}
			if k.Type == tea.KeySpace && len(k.Runes) == 0 {
				m.gitBuf = append(m.gitBuf, ' ')
			}
		}
		return m, nil
	}
	switch k.Type {
	case tea.KeyEnter:
		return m.saveGitKey()
	case tea.KeyBackspace:
		if len(m.gitName) > 0 {
			m.gitName = m.gitName[:len(m.gitName)-1]
		}
	case tea.KeyRunes:
		if !k.Paste {
			m.gitName = append(m.gitName, k.Runes...)
		} else {
			m.gitName = append(m.gitName, []rune(strings.TrimSpace(string(k.Runes)))...)
		}
	}
	return m, nil
}

func (m model) saveGitKey() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(string(m.gitName))
	if len(sshkey.Normalize(m.gitBuf)) == 0 {
		m.gitNote = "請先貼上私鑰"
		m.gitFocus = 0
		return m, nil
	}
	if err := sshkey.ValidName(name); err != nil {
		m.gitNote = err.Error()
		m.gitFocus = 1
		return m, nil
	}
	path, err := m.sshLayout().SaveKey(name, m.gitBuf)
	if err != nil {
		m.gitNote = err.Error()
		return m, nil
	}
	m.clearGitInput()
	return m.applyGitKey(name, path)
}

func (m model) applyGitKey(name, saved string) (tea.Model, tea.Cmd) {
	l := m.sshLayout()
	now := time.Now()
	if m.deps.Now != nil {
		now = m.deps.Now()
	}
	ident := l.Identity(name)
	res, err := l.Apply(ident, now)
	if err != nil {
		m.gitNote = "寫入 ~/.ssh/config 失敗：" + err.Error()
		return m, nil
	}
	lines := []string{}
	if saved != "" {
		lines = append(lines, "已儲存私鑰 "+ident+"（權限 600）")
	}
	if res.Changed {
		lines = append(lines, "已設定 ~/.ssh/config：github.com 使用 "+ident)
	} else {
		lines = append(lines, "~/.ssh/config 已經是 "+ident+"，沒有變更")
	}
	if res.Backup != "" {
		lines = append(lines, "原本的設定備份在 "+shortHome(res.Backup, m.home()))
	}
	if res.Unmanaged {
		lines = append(lines, "注意：config 裡原本就有 Host github.com；新的區塊放在最上面，ssh 會優先使用它")
	}
	lines = append(lines, "", "測試連線：ssh -T git@github.com")
	m.gitDoneLines = lines
	m.gitStage = gitDone
	m.gitNote = ""
	return m, nil
}

func keyHeader(buf []byte) string {
	s := strings.TrimLeft(string(buf), " \t\r\n")
	line, _, _ := strings.Cut(s, "\n")
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "-----BEGIN ") && strings.HasSuffix(line, "-----") {
		return line
	}
	return ""
}

func gitKeyBody(m model) []string {
	l := m.sshLayout()
	dir := strings.TrimSuffix(l.IdentRel, "/")
	lines := []string{"設定 GitHub SSH 鑰匙", ""}
	switch m.gitStage {
	case gitDone:
		lines = append(lines, m.gitDoneLines...)
	case gitPick:
		lines = append(lines, "選一把 "+dir+" 裡的私鑰給 github.com 使用：")
		if cur := sshkey.Current(l.ConfigPath()); cur != "" {
			lines = append(lines, "目前設定："+cur)
		}
		lines = append(lines, "")
		for i, name := range append(append([]string(nil), m.gitKeys...), gitAddNew) {
			mark := "  "
			if i == m.gitCursor {
				mark = "> "
			}
			lines = append(lines, mark+name)
		}
	case gitAdd:
		if len(m.gitKeys) == 0 {
			lines = append(lines, dir+" 裡沒有私鑰，請貼上私鑰並取名。")
		} else {
			lines = append(lines, "貼上新的私鑰並取名。")
		}
		lines = append(lines, "私鑰只存成 "+dir+"/<名稱>（權限 600），不會顯示、記錄或上傳。", "")
		mark := func(f int) string {
			if m.gitFocus == f {
				return "> "
			}
			return "  "
		}
		state := "（尚未貼上）"
		if n := len(m.gitBuf); n > 0 {
			state = fmt.Sprintf("已貼上 %d 行、%d 字元", strings.Count(strings.TrimRight(string(m.gitBuf), "\n"), "\n")+1, utf8.RuneCount(m.gitBuf))
			if h := keyHeader(m.gitBuf); h != "" {
				state += "  " + h
			}
		}
		lines = append(lines, mark(0)+"私鑰："+state)
		lines = append(lines, mark(1)+"名稱："+string(m.gitName))
	}
	if m.gitNote != "" {
		lines = append(lines, "", m.gitNote)
	}
	return lines
}

func gitKeyHints(m model) []hint {
	switch m.gitStage {
	case gitDone:
		return []hint{{Key: "操作"}, {Key: "任意鍵", Desc: "回主頁"}}
	case gitPick:
		return []hint{{Key: "操作"}, {Key: "↑/↓", Desc: "選擇"}, {Key: "enter", Desc: "使用"}, {Key: "esc", Desc: "略過"}}
	default:
		return []hint{{Key: "操作"}, {Key: "tab", Desc: "切換欄位"}, {Key: "enter", Desc: "儲存（名稱欄）"}, {Key: "ctrl+s", Desc: "儲存"}, {Key: "ctrl+u", Desc: "清除"}, {Key: "esc", Desc: "返回"}}
	}
}

func gitKeyHelp(m model) string {
	parts := []string{}
	for _, h := range gitKeyHints(m)[1:] {
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return "操作  " + strings.Join(parts, "  ")
}
