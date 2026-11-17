package pm

import "github.com/shuhi/dotfiles/internal/catalog"

var managerOrder = []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "xbps", "brew"}

var binaries = map[string]string{
	"apt":    "apt-get",
	"dnf":    "dnf",
	"yum":    "yum",
	"pacman": "pacman",
	"zypper": "zypper",
	"apk":    "apk",
	"xbps":   "xbps-install",
	"brew":   "brew",
}

func Managers() []string {
	return append([]string(nil), managerOrder...)
}

func Binary(id string) (string, bool) {
	b, ok := binaries[id]
	return b, ok
}

func Known(id string) bool {
	_, ok := binaries[id]
	return ok
}

const (
	MakeMissingMessage     = "macOS 的 make 是 /usr/bin/make，由 Xcode Command Line Tools 提供。Homebrew formula make 只安裝 gmake，本工具不安裝它。請改勾 compiler。"
	XcodePendingMessage    = "已要求安裝 Xcode Command Line Tools，完成對話框後請再執行一次"
	NetToolsMissingMessage = "macOS 沒有對應的 net-tools formula"
	GoNoRootMessage        = "發行版沒有 go 套件，且沒有權限寫入 /usr/local/go"
	EPELMessage            = "可能需要 EPEL，本工具不會啟用它"
	TrashShadowNote        = "keg-only，系統 /usr/bin/trash 仍在預設 PATH；trash-cli 在 Homebrew prefix 的 opt/trash-cli/bin/trash。本工具不執行 brew link --force，也不改 bashrc。"
	KegOnlyGenericNote     = "keg-only，預設 PATH 仍使用系統命令。本工具不執行 brew link --force。"
)

func KegOnlyNote(id catalog.ID, manager string, method Method) string {
	if manager != "brew" || method != MethodPackage {
		return ""
	}
	switch id {
	case "curl", "dnsutils", "whois":
		return KegOnlyGenericNote
	case "trash":
		return TrashShadowNote
	default:
		return ""
	}
}
