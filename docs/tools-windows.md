# Windows（for_windows 分支）

在 Windows 上執行 `tools.exe`（Windows Terminal、PowerShell 或 cmd 都可以）會出現同一個 TUI，但清單只有 Windows 版的三項，全部用 [Scoop](https://scoop.sh) 安裝，以一般使用者身分執行即可，不需要系統管理員。

## 清單

| 項目 | 安裝 | 移除 |
|------|------|------|
| wezterm | `scoop bucket add extras` → `scoop install extras/wezterm` | `scoop uninstall wezterm` |
| python | `scoop install python` | `scoop uninstall python` |
| git | `scoop install git` | `scoop uninstall git`，再加 `ssh-config git` 步驟 |

三項預設都勾選。Linux／macOS 的其他工具不會出現在 Windows 清單裡。

## 安裝流程

1. **Scoop**：第一個步驟永遠是 `scoop`。找到 `%SCOOP%\shims\scoop.cmd`（沒設 `SCOOP` 就是 `~\scoop\shims\scoop.cmd`）、`scoop.ps1` 或 PATH 上的 `scoop` 就略過（已安裝）；否則用官方安裝指令：

   ```
   powershell -NoProfile -ExecutionPolicy Bypass -Command "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force; irm get.scoop.sh | iex"
   ```

   這一步失敗（例如用系統管理員身分執行，Scoop 預設拒絕）會停止整個流程並在結果頁顯示原因，後面的套件不會執行。剛裝好的 Scoop 還不在這個程式的 PATH 上，所以之後的 `scoop …` 會直接呼叫 `shims\scoop.cmd`。
2. **extras bucket**：有勾 wezterm 時加一步 `scoop bucket add extras`。`~\scoop\buckets\extras` 已存在就略過；Scoop 回報 "already exists" 也算成功。
3. **套件**：依清單順序 `scoop install …`。找到 `~\scoop\shims\<名稱>.exe` 或 PATH 上已有同名指令（例如另外裝的 Git for Windows）就標成「已安裝」不重裝；`--force` 會重新執行。
4. 沒有 `sudo`、沒有套件索引更新步驟，也不寫 `~/.bashrc`、`~/.zshrc`、starship 等設定檔，這些都只在 Linux／macOS 做。
5. git 就緒後接著 GitHub SSH 鑰匙設定（見 `docs/tools-git-ssh.md`）：鑰匙直接放在 `%USERPROFILE%\.ssh\`，config 寫 `IdentityFile ~/.ssh/<鑰匙名稱>`，新鑰匙用 `icacls` 把權限收成只有自己可讀寫（盡力而為，失敗不擋流程）。

進度頁、`d` 計畫頁（白色清單，`scoop` 那行顯示「沒有 Scoop 時執行：powershell …」）、`ctrl+c` 取消都跟 Linux 一樣。取消時 Windows 沒有 SIGTERM，會直接結束子程序。

## 移除

移除畫面只列出已安裝的那幾項（Windows 沒有鎖定的「基礎」類，三項都能移除），每項一個 `scoop uninstall`。移除 git 時多一步 `ssh-config git`：只刪 `~/.ssh/config` 裡的 `dotfiles:github` 區塊，鑰匙檔保留。python 的 dnf 警告只在 Linux 顯示。

## 偵測

- 平台：`GOOS=windows` 直接把套件管理員定為 `scoop`（還沒裝也一樣，安裝流程會先裝），不讀 `/etc/os-release`。
- 已安裝：`%SCOOP%\shims\<名稱>.exe`（或 `~\scoop\shims\…`）存在，或 `exec.LookPath` 找得到。

## 不支援

- `tools apply`（非互動模式）在 Windows 會直接回報不支援，請用互動介面。

## 下載

GoReleaser 會建 `tools_<版本>_windows_amd64.zip` 與 `tools_<版本>_windows_arm64.zip`，裡面是 `tools.exe` 與 README。這個分支不打 tag、不發佈。
