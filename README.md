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
- 「容器」分類有 docker 與 podman。docker 依系統用不同套件（Fedora `moby-engine docker-compose`、Debian／Ubuntu `docker.io`、Arch／openSUSE／Void `docker docker-compose`、Alpine `docker docker-cli-compose`、mac `docker docker-compose` 只有 CLI，需要 Docker Desktop 或 colima）。Linux 裝完會加入 docker 群組，有 systemd 時啟用並啟動服務（沒開 systemd 的 WSL 會跳過並提示）。移除只移套件，不刪映像檔和 volume。詳見 `docs/tools-containers.md`。
- 「語言」分類有 zig：Fedora／Arch／openSUSE／Alpine／Void／mac 用套件，Debian／Ubuntu 用 ziglang.org 官方 tarball（核對 sha256 與 minisign 簽章），裝在 `~/.local/share/zig`，`~/.local/bin/zig` 是 symlink。詳見 `docs/tools-zig.md`。fastfetch 在 Shell 分類。
- `d` 只顯示計畫（白色清單，不執行），安裝和移除畫面都有，按任意鍵回到原本的清單，勾選保留。`a` 切換游標所在分類、`A` 切換全部：已安裝的工具不會被一起勾，但如果範圍內全都已安裝，就切換它們（會重新下載），狀態列會提示。
- 安裝跑完、git 已就緒時，會接著設定 GitHub SSH 鑰匙：從 `~/.ssh/keys/` 選一把，或貼上新的私鑰並取名（存成 600），再把 `Host github.com` 的受管理區塊寫進 `~/.ssh/config`（第一次會先備份），`esc` 略過。詳見 `docs/tools-git-ssh.md`。
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

設計文件在 `docs/`，最新的是 `docs/tools-git-ssh.md`、`docs/tools-zig.md`、`docs/tools-containers.md`、`docs/tools-progress-design.md` 與 `docs/tools-uninstall-design.md`。
