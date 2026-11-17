# `tools` 解除安裝、設定檔部署、bash/zsh 設定區塊與首頁色系

| 欄位 | 值 |
| --- | --- |
| 日期 | 2026-11-17 |
| 狀態 | Implemented |
| 模組 | `github.com/shuhi/dotfiles` |
| 取代範圍 | `tools-home-design.md` 的 `phaseUnavailable`（「此動作不可用」）與「不做解除安裝」；`tools-installed-skip-design.md` 的「不做解除安裝」；`tools-tui-earth-design.md` 的目錄色票（Andromeda／earth）；`tools-installer-design.md` 中 link-bash 只寫 `~/.bashrc` 的部分。其他段落不變 |

## 1. 目錄色系（tools-download／tools-uninstall）

所有畫面（首頁、目錄、confirm、安裝中、報告、解除安裝、python 警告、強制重裝問句、頂端系統列、底欄）**都不畫背景色**，只用文字顏色，終端機底色透出來；輸出裡不會有 `48;2` 或反白 SGR（`TestNoScreenEmitsBackground` 檢查）。文字顏色沿首頁的標題漸層 `#F0ADA0 → #9EE2F0`（`lerpTitle`）。

| token | 值 | 用途 |
| --- | --- | --- |
| 游標列 | `#F0ADA0`（`selectColor`） | 整列文字變桃色，前面是粗體 `>`；沒有填色 |
| 首頁選中選單 | `#F0ADA0` 粗體 | 同上，沒有填色 |
| 頂端系統列 | 漸層、粗體 | 逐字走漸層 |
| `(已安裝)` | `#9EE2F0` | 已安裝列右緣標記 |
| `clay` | `#F0ADA0` | 已勾選 `[x]` 與名稱 |
| `sand` | `#D9CFCD` | 未勾選 |
| `gold` | `#F7C6BA` | 預設 `*` |
| `key` | `#9EE2F0` | 底欄按鍵 |
| `cream` | `#EDE3E1` | 一般文字、強制重裝問句 |
| `muted` | `#9C8F92` | 說明文字、狀態列 |
| 分類標題 | `catColor(i, n)` | 依分類順序沿漸層由桃到青 |

## 2. 返回首頁按鍵

瀏覽畫面新增 `Back` 綁定：`esc` 或 `b`，底欄顯示 `esc/b 返回首頁`。其他按鍵不變。

## 3. tools-uninstall

首頁第二項改為真的解除安裝流程（`phaseUninstall`）：

1. 清單只列「已安裝」且可移除的工具，依目錄分類與順序；不顯示 `(已安裝)` 樣式。
2. **基礎**分類（git、curl、wget、unzip、rsync、vim、less、net-tools、whois、dnsutils、traceroute）不可移除，不列出；狀態列寫明。
3. 由系統提供、無法撤銷的方法（Xcode CLT、`@pre` 預裝檔）不列出。
4. 按鍵：`↑↓/jk`、`space`、`a`、`A`、`g`、`G`、`enter` 移除已勾選、`esc/b` 返回首頁、`q` 離開。
5. 勾選 python 時先進 `phasePythonWarn`：警告 Fedora 的 dnf 依賴 python3；只有 `y` 繼續，`n`/`esc` 返回。
6. confirm 畫面列出所有步驟，`y` 執行，`n/esc` 回清單；報告畫面任意鍵回清單。

`internal/uninstall` 依實際留下的檔案決定步驟（同一工具可能有多個）：

| 方法 | 偵測 | 動作 |
| --- | --- | --- |
| 套件 | 套件的 probe 命中；都沒命中時仍交給套件管理器 | `pm.BuildRemove`：`apt-get remove -y`、`dnf/yum remove -y`、`pacman -R --noconfirm`、`zypper --non-interactive remove`、`apk del`、`xbps-remove -y`、`brew uninstall`；非 root 且非 brew 加 `sudo`（與安裝相同的 `WithPrivilege`）。apt 的 fd/bat 另刪指向 `/usr/bin/fdfind`、`/usr/bin/batcat` 的 `~/.local/bin` symlink |
| GitHub release | `~/.local/bin/<bin>` 存在 | 刪該檔；若是指向 `~/.local/share/<sub>/<top>/…` 的 symlink，刪 `<top>` 樹，`~/.local/share/<sub>` 只在空了才刪（neovim 的使用者資料也在那裡） |
| Go tarball | `/usr/local/go/bin/go` | root：程序內 `RemoveAll`；非 root：`sudo rm -rf /usr/local/go`（argv 精確比對） |
| rustup | `~/.cargo/bin/rustup` | `~/.cargo/bin/rustup self uninstall -y` |

之後依序：移除該工具在 `~/.bashrc` 與 `~/.zshrc` 的設定區塊，再處理設定檔。任何一步失敗，該工具後面的步驟略過（套件沒移掉就不動它的設定）。

設定檔移除（`configcopy.Undeploy`）：內容和 repo 相同就刪；被改過的檔改名成 `.bak.<UTC 時間>`；`~/.zshrc` 這類「只在不存在時複製」的檔案若被改過則保留。刪檔後清掉空目錄，不越過 `~/.config`／home。

## 4. 設定檔部署

安裝成功（含 satisfied）後，`config <id>` 步驟把 repo 的設定部署出去（`configcopy.FilesFor`）：

| 工具 | 來源 | 目的地 | 政策 |
| --- | --- | --- | --- |
| starship | `starship/starship.toml`（主題；沒有時退回 `shells/starship/starship.toml`） | `$XDG_CONFIG_HOME` 或 `~/.config/starship.toml` | 不同就備份 `.bak.<UTC>` 再取代 |
| zellij | `zellij/` 底下所有檔案（`config.kdl`、`layouts/*`、`plugins/zjstatus.wasm`） | `~/.config/zellij/…` | 同上 |
| zsh | `shells/zsh/zshrc` | `~/.zshrc` | 只在不存在時複製 |

`ghostty/`、`wezterm/` 屬於目錄不安裝的終端機程式；`WSL/` 是空的，不部署。

## 5. bash 與 zsh 設定區塊

`internal/shellrc` 取代 Runner 原本只寫 `~/.bashrc` 的 link-bash；步驟名稱改為 `shell-rc`。不論預設 shell 是哪個，`~/.bashrc` 與 `~/.zshrc` 都寫同一組區塊，各用自己的語法：

```sh
# >>> dotfiles:starship >>>
if command -v starship >/dev/null 2>&1; then
  eval "$(starship init zsh)"      # bash 檔裡是 init bash
fi
# <<< dotfiles:starship <<<
```

| 區塊 | 何時加 | 內容 |
| --- | --- | --- |
| `local-bin` | 有 release 安裝 | `~/.local/bin` 進 PATH（多個工具共用，解除安裝不刪） |
| `starship` | starship 成功 | `starship init bash/zsh` |
| `fzf` | fzf 成功 | bash `eval "$(fzf --bash)"`／zsh `source <(fzf --zsh)` |
| `rust` | rustup 安裝成功 | `. "$HOME/.cargo/env"` |
| `go` | Go tarball 成功 | `/usr/local/go/bin` 進 PATH |

寫入是冪等的：所有受管區塊以固定順序放在檔尾（local-bin 在最前），重跑內容不變；舊版 `# dotfiles:<hook>` 片段會被換成新區塊。檔案不存在就建立（0644），既有權限保留，symlink 與 home 以外的路徑拒絕。`cmd/link-bash` 仍是舊的只寫 bash 的獨立工具。
