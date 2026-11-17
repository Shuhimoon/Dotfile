package catalog

type ID string

type Category struct {
	ID    string
	Title string
	Items []Item
}

type Item struct {
	ID          ID
	Name        string
	Default     bool
	Description string
}

func All() []Category {
	return []Category{
		{ID: "basics", Title: "基礎", Items: []Item{
			item("git", true, ""),
			item("curl", true, ""),
			item("wget", true, "Fedora 套件是 wget2-wget"),
			item("unzip", true, ""),
			item("rsync", true, ""),
			item("vim", true, ""),
			item("less", true, ""),
			item("net-tools", true, "ifconfig, netstat"),
			item("whois", true, ""),
			item("dnsutils", true, "dig, nslookup"),
			item("traceroute", true, ""),
		}},
		{ID: "shell", Title: "Shell", Items: []Item{
			item("zsh", false, ""),
			item("starship", false, ""),
			item("fastfetch", false, ""),
		}},
		{ID: "files", Title: "檔案搜尋", Items: []Item{
			item("eza", true, ""),
			item("trash", true, "trash-cli"),
			item("ripgrep", true, ""),
			item("fd", true, "Debian 上命令是 fdfind"),
			item("bat", true, ""),
			item("jq", true, ""),
			item("fzf", true, ""),
		}},
		{ID: "editors", Title: "編輯器", Items: []Item{
			item("neovim", false, ""),
			item("zellij", true, ""),
			item("mitos", true, "執行檔 ms"),
		}},
		{ID: "languages", Title: "語言", Items: []Item{
			item("go", false, ""),
			item("rust", false, ""),
			item("python", false, ""),
			item("node", false, ""),
			item("zig", false, "Debian/Ubuntu 用 ziglang.org 官方版"),
		}},
		{ID: "build", Title: "編譯", Items: []Item{
			item("make", false, ""),
			item("cmake", false, ""),
			item("compiler", false, "平台 C/C++ 編譯器"),
			item("pkg-config", false, "pkg-config 或 pkgconf"),
		}},
		{ID: "containers", Title: "容器", Items: []Item{
			item("docker", false, "Linux 會加入 docker 群組並啟動服務；mac 只裝 CLI"),
			item("podman", false, "免 daemon、可 rootless"),
		}},
	}
}

func item(id string, def bool, desc string) Item {
	return Item{ID: ID(id), Name: id, Default: def, Description: desc}
}

func ItemByID(id ID) (Item, bool) {
	for _, c := range append(All(), Windows()...) {
		for _, it := range c.Items {
			if it.ID == id {
				return it, true
			}
		}
	}
	return Item{}, false
}

// DefaultIDs is the fixed default selection. It is not derived from Item.Default.
func DefaultIDs() []ID {
	return []ID{
		"git", "curl", "wget", "unzip", "rsync", "vim", "less",
		"net-tools", "whois", "dnsutils", "traceroute",
		"eza", "trash", "ripgrep", "fd", "bat", "jq", "fzf",
		"zellij", "mitos",
	}
}

func IDs() []ID {
	var ids []ID
	for _, c := range All() {
		for _, it := range c.Items {
			ids = append(ids, it.ID)
		}
	}
	return ids
}
