# sudo、shell rc 與 zsh 範本

## 問題（2026-11-18，WSL Fedora_test）

`sudo ./tools` 之後 `~/.bashrc` 沒變、沒有 `~/.zshrc`、`starship`/`zellij` 不在 PATH、
`~/.local/bin` 不存在，但 rpm 套件有裝好。

根本原因：sudo 會把 HOME 設成 `/root`。tools 用 `os.UserHomeDir()`（讀 HOME），
所以 release 工具（starship、zellij 在 dnf 上是 `@rel`，裝到 `~/.local/bin`）、
設定檔、rc 區塊、log 全部寫進 `/root`。系統套件本來就是全域的，所以正常。

非 sudo 的流程沒有另外的 bug，但有一個缺口：rc 區塊只依「這次的結果」產生，
starship 只在這次裝好或已滿足時才寫 `starship init`；已安裝的項目預設不勾，
重跑就不會補寫。現在每次安裝都會寫 `dotfiles:base` 區塊。

## sudo（`internal/sudouser`）

- `Detect(euid, getenv, lookup)`：euid 0 且 `SUDO_USER` 不是空的也不是 root，
  才算 sudo 執行；uid/gid 取 `SUDO_UID`/`SUDO_GID`，沒有就用 passwd。
  uid 0、家目錄不是絕對路徑或是 `/` 都拒絕（exit 2）。
- `Target.Env`：把 HOME 設成本人家目錄（子行程如 rustup-init 也跟著），
  `XDG_CONFIG_HOME` 指到家目錄外（root 的）就移除。
- `Fixer.Fix(Roots(home))`：走 `~/.local ~/.config ~/.cargo ~/.rustup ~/.ssh ~/.zsh`
  與 `~/.bashrc* ~/.zshrc*`，只把 uid 0 的項目 lchown 給本人；不跟隨 symlink，
  別人擁有的檔不動。在解出設定檔後、`apply` 跑完後、TUI 每次安裝結束與
  GitHub 鑰匙存檔後執行。
- 套件指令：`pm.WithPrivilege` 在 euid 0 時本來就不加 sudo，所以不會出現密碼頁。
- 直接 root 登入（沒有 `SUDO_USER`）不變，用 `/root`。
- macOS 上 brew 不能用 root 跑，`sudo ./tools` 在 mac 不支援。

## shell rc

- `dotfiles:base`（`shellrc.BlockBase`）一律寫進 `~/.bashrc` 與 `~/.zshrc`
  （缺檔就建立）：`GOPATH` 預設、把存在的 `$GOPATH/bin`、`/usr/local/go/bin`、
  `~/.local/bin` 加到 PATH 前面（已在 PATH 就跳過）、`~/.cargo/env`、eza alias，
  最後 `starship init bash|zsh`（`command -v starship` 才跑）。
- base 涵蓋 `local-bin`、`starship`、`rust`、`go` 區塊，有 base 就不再寫那些。
- 檔案裡有 `# dotfiles:provides <ids>` 行時，那些區塊不寫（zsh 範本用它宣告
  自己已經處理 PATH、starship…）。fzf 等其他區塊照常附加在最後。

## zsh 範本（`shells/zsh/zshrc`）

來源是 Shuhi 的 Mac `~/.zshrc`，拿掉 grok 安裝區塊、flutter、Android/JAVA_HOME、
`/Users/...`、結尾的 `go env` 行；外掛 `source` 都先檢查檔案，starship 先檢查指令。

- 部署策略 `configcopy.PolicyZshrc`：舊檔去掉 dotfiles 區塊後跟範本一樣就不動；
  不一樣就備份成 `.zshrc.bak.<UTC>`，寫入範本，再把舊檔裡的 dotfiles 區塊加回去。
- 解除安裝：去掉區塊後跟範本一樣才刪，改過就保留。
- 外掛：`zsh-plugins` 步驟（在 config zsh 之後）在 zsh 裝好或已存在時建立
  `~/.zsh/cache`，`git clone --depth 1` 三個外掛到 `~/.zsh/<名稱>`，已存在就略過。
  解除安裝 zsh 刪掉這三個資料夾與 cache，`~/.zsh` 空了才刪。

## zellij

在 box（glibc x86_64）實際下載 v0.46.0：選到 `zellij-x86_64-unknown-linux-gnu.tar.gz`，
`zellij --version` 正常，repo 的設定 `zellij setup --check` 通過。Fedora x86_64 也是 glibc，
走同一個資產。Shuhi 看到的錯誤是 sudo 把 zellij 裝進 `/root/.local/bin`，本人的 PATH 找不到。

另外修了一個潛在 bug：v0.46.0 起 musl 與 macOS 各有 `zellij-no-web-<triple>.tar.gz`，
跟完整版同時符合會報「多個資產符合」。現在兩者都在時選完整版。
