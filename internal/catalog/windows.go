package catalog

// Windows is the catalog shown on Windows. Every item is installed with Scoop.
func Windows() []Category {
	return []Category{
		{ID: "windows", Title: "Windows（Scoop）", Items: []Item{
			item("wezterm", true, "scoop extras bucket"),
			item("python", true, ""),
			item("git", true, "裝完後設定 GitHub SSH 鑰匙"),
		}},
	}
}

// ForOS is the catalog for goos: Windows() on windows, All() elsewhere.
func ForOS(goos string) []Category {
	if goos == "windows" {
		return Windows()
	}
	return All()
}

// IDsFor lists the ids of ForOS(goos) in display order.
func IDsFor(goos string) []ID {
	var ids []ID
	for _, c := range ForOS(goos) {
		for _, it := range c.Items {
			ids = append(ids, it.ID)
		}
	}
	return ids
}
