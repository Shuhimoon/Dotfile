package tui

import (
	"path/filepath"
	"strings"

	"github.com/shuhi/dotfiles/internal/install"
	"github.com/shuhi/dotfiles/internal/shellrc"
	"github.com/shuhi/dotfiles/internal/uninstall"
)

// The d key shows what enter would do, in white, without running anything.
// Any key then returns to the list it came from with the ticks kept.

func installPlan(steps []install.Step, home string) []string {
	out := make([]string, 0, len(steps))
	for _, st := range steps {
		out = append(out, st.Title+"  "+installDetail(st, home))
	}
	return out
}

func installDetail(st install.Step, home string) string {
	var s string
	switch {
	case st.Kind == install.KindScoopBootstrap:
		s = "沒有 Scoop 時執行：" + strings.Join(st.Argv, " ")
	case len(st.Argv) > 0:
		s = strings.Join(st.Argv, " ")
	case st.Kind == install.KindRelease:
		s = "下載 " + st.ReleaseRepo + " 到 ~/.local/bin"
	case st.Kind == install.KindService:
		s = "groupadd -f docker；usermod -aG docker 你的帳號；有 systemd 時 systemctl enable --now docker"
	case st.Kind == install.KindConfig:
		s = "部署 " + string(st.ItemID) + " 設定檔（不同的既有檔先備份）"
	case st.Kind == install.KindLink:
		s = "在 ~/.bashrc、~/.zshrc 寫入受管理區塊（PATH、cargo、eza alias、starship init）"
	case st.Kind == install.KindZshPlugins:
		s = "git clone fzf-tab、zsh-autosuggestions、zsh-syntax-highlighting 到 ~/.zsh/（已存在就略過）"
	case st.Kind == install.KindGo:
		s = "下載 go.dev 官方 tarball 到 /usr/local/go"
	case st.Kind == install.KindRustup:
		s = "下載並執行 rustup-init"
	case st.Kind == install.KindXcode:
		s = "xcode-select --install"
	default:
		s = string(st.Kind)
	}
	if fb := st.Fallback; fb != nil {
		alt := installDetail(*fb, home)
		s += "（找不到套件時改用：" + alt + "）"
	}
	return shortHome(s, home)
}

func uninstallPlan(steps []uninstall.Step, home string) []string {
	out := make([]string, 0, len(steps))
	for _, st := range steps {
		out = append(out, st.Title+"  "+shortHome(uninstallDetail(st), home))
	}
	return out
}

func uninstallDetail(st uninstall.Step) string {
	var parts []string
	if len(st.Argv) > 0 {
		parts = append(parts, strings.Join(st.Argv, " "))
	}
	switch st.Kind {
	case uninstall.KindRelease:
		parts = append(parts, "刪除 "+st.Bin)
		if st.ShareDir != "" {
			parts = append(parts, "刪除 "+filepath.Join(st.ShareDir, "<版本目錄>"))
		}
	case uninstall.KindGo:
		if len(st.Argv) == 0 {
			parts = append(parts, "刪除 "+st.GoDir)
		}
	case uninstall.KindShell:
		var b []string
		for _, id := range st.Blocks {
			b = append(b, shellrc.Begin(id))
		}
		parts = append(parts, "從 ~/.bashrc、~/.zshrc 刪除 "+strings.Join(b, "、")+" 區塊")
	case uninstall.KindZshPlugins:
		parts = append(parts, "刪除 ~/.zsh/ 裡的 fzf-tab、zsh-autosuggestions、zsh-syntax-highlighting 與 cache")
	case uninstall.KindSSH:
		parts = append(parts, "從 ~/.ssh/config 刪除 # >>> dotfiles:github >>> 區塊（鑰匙檔保留）")
	case uninstall.KindConfig:
		parts = append(parts, "移除部署的 "+string(st.ItemID)+" 設定檔（改過的先備份或保留）")
	}
	for _, l := range st.Links {
		parts = append(parts, "刪除 "+l)
	}
	if len(parts) == 0 {
		return string(st.Kind)
	}
	return strings.Join(parts, "；")
}

func shortHome(s, home string) string {
	if home == "" || home == "/" {
		return s
	}
	return strings.ReplaceAll(s, home+"/", "~/")
}

func dryBackDesc(m model) string {
	if m.mode == modeUninstall {
		return "回移除畫面"
	}
	return "回安裝畫面"
}
