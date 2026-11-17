package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Install key.Binding
	CatAll  key.Binding
	All     key.Binding
	Dry     key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Quit    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "上一項")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "下一項")),
		Toggle:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "勾選/取消")),
		Install: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "安裝已勾選")),
		CatAll:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "切換本類")),
		All:     key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "切換全部")),
		Dry:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "只顯示計畫")),
		Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "頂端")),
		Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "底端")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "離開")),
	}
}

func (k keyMap) browsing() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Install, k.CatAll, k.All, k.Dry, k.Top, k.Bottom, k.Quit}
}

func browsingHelp(k keyMap) string {
	parts := make([]string, 0, 10)
	for _, b := range k.browsing() {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return "操作  " + strings.Join(parts, "  ")
}

const (
	confirmHelp    = "操作  y 開始  n/esc 返回"
	installingHelp = "操作  ctrl+c 取消"
	reportHelp     = "操作  任意鍵返回  q 離開"
)
