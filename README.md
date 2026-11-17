# Dotfiles

個人 dotfiles 與 `tools`：一個 Bubble Tea TUI，用來在 Linux／macOS 安裝與移除常用 CLI 工具，並部署這個 repo 裡的設定檔。

```sh
go run ./cmd/tools            # TUI：tools-download / tools-uninstall
go run ./cmd/tools apply --only starship,zellij --dry-run
go test ./...
```

- 設定檔：`starship/`、`zellij/`、`shells/zsh/`，安裝成功後部署到 `~/.config` 與 `~/.zshrc`（不同的既有檔先備份成 `.bak.<UTC>`）。
- shell 設定：`~/.bashrc` 與 `~/.zshrc` 都會寫入 `# >>> dotfiles:<tool> >>>` 區塊，解除安裝時只刪那幾行。
- 「基礎」分類（git、curl、vim…）不可解除安裝；移除 python 前會先警告。

設計文件在 `docs/`，最新的是 `docs/tools-uninstall-design.md`。
