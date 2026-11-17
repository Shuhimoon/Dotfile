# Dotfiles

個人 dotfiles 與 `tools`：使用 Bubble Tea TUI 框架， 用來在 Linux／macOS 安裝與移除常用 CLI 工具 。

```sh
go run ./cmd/tools            # TUI：tools-download / tools-uninstall
go run ./cmd/tools apply --only starship,zellij --dry-run
go test ./...
```

- 設定檔：`starship/`、`zellij/`、`shells/zsh/`，安裝成功後部署到 `~/.config` 與 `~/.zshrc`（不同的既有檔先備份成 `.bak.<UTC>`）。
- shell 設定：`~/.bashrc` 與 `~/.zshrc` 都會寫入 `# >>> dotfiles:<tool> >>>` 區塊，解除安裝時只刪那幾行。
- 「基礎」分類（git、curl、vim…）不可解除安裝；移除 python 時進度頁會顯示警告。
- 安裝與移除都留在 TUI 裡：`enter` 直接開始（沒有 y 確認），進度頁顯示步驟狀態與即時輸出，`ctrl+c` 取消，結束後任意鍵回首頁。需要 sudo 時在 TUI 裡輸入密碼（遮罩，只交給 sudo 驗證）。

## 下載執行檔

[GitHub Releases](https://github.com/Shuhimoon/Dotfile/releases) 有 Linux（amd64、arm64、armv7、386）與 macOS（Intel、Apple Silicon）的 `tools_<版本>_<os>_<arch>.tar.gz`，解開後只有 `tools` 與這份 README。私人 repo 可用 gh 下載：

```sh
gh release download v0.1.0 -R Shuhimoon/Dotfile -p "tools_*_linux_amd64.tar.gz"
tar xzf tools_*_linux_amd64.tar.gz && ./tools --version
```

設定檔（`starship/`、`zellij/`、`shells/zsh/zshrc`、`shells/starship/`）用 `go:embed` 編進執行檔（`configs.go`），不用帶 repo。在這個 repo 的 checkout 裡執行時（`go.mod` 的 module 是 `github.com/shuhi/dotfiles`）直接用 checkout 的檔案；其他地方則把內嵌的副本解到 `~/.local/share/dotfiles-tools/configs/<版本>/` 再部署（內容相同的檔不重寫）。

## 發佈（本機 GoReleaser，不用 GitHub Actions）

```sh
goreleaser check
goreleaser release --snapshot --clean          # 只建置到 dist/
git tag v0.2.0 && git push origin v0.2.0
GITHUB_TOKEN="$(gh auth token)" goreleaser release --clean
```

版本、commit、建置時間用 ldflags 寫進 `main.version`／`main.commit`／`main.date`，`tools --version` 會顯示。

設計文件在 `docs/`，最新的是 `docs/tools-progress-design.md` 與 `docs/tools-uninstall-design.md`。
