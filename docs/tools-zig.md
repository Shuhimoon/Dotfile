# zig（語言分類）

「語言」分類最後一項是 `zig`，預設不勾選，可以移除。依系統分開安裝：

| 系統 | 管理器 | 方式 | 偵測 |
|------|--------|------|------|
| Debian／Ubuntu（含 WSL） | apt | ziglang.org 官方 tarball | `~/.local/bin/zig` |
| CentOS 7／Amazon Linux 2 | yum | ziglang.org 官方 tarball | `~/.local/bin/zig` |
| Fedora（含 WSL Fedora） | dnf | `sudo dnf install -y zig`，找不到套件時改用 tarball | `/usr/bin/zig` 或 `~/.local/bin/zig` |
| Arch | pacman | `sudo pacman -S --needed --noconfirm zig`，失敗同上 | 同上 |
| openSUSE | zypper | `sudo zypper --non-interactive install --no-recommends zig`，失敗同上 | 同上 |
| Alpine | apk | `sudo apk add zig`，失敗同上 | 同上 |
| Void | xbps | `sudo xbps-install -y -S zig`，失敗同上 | 同上 |
| macOS | brew | `brew install zig`，失敗同上 | `/opt/homebrew/bin/zig`、`/usr/local/bin/zig` 或 `~/.local/bin/zig` |

Debian／Ubuntu 沒有可靠、夠新的 zig 套件，所以直接用官方版。

## 官方 tarball（`ziglang.org/zig`）

1. 下載 `https://ziglang.org/download/index.json`，選最新的 stable 版本（版本號是 `X.Y.Z`，跳過 `master` 與 dev 版）。
2. 依平台選檔：`x86_64-linux`、`aarch64-linux`、`x86-linux`（舊版叫 `i386-linux`）、`arm-linux`（舊版 `armv7a-linux`）、`riscv64-linux`、`powerpc64le-linux`、`s390x-linux`、`loongarch64-linux`、`x86_64-macos`、`aarch64-macos`。下載網址必須在 `https://ziglang.org/` 底下、以 `.tar.xz` 結尾。
3. 核對 index.json 給的 sha256。
4. 下載同名的 `.minisig`，用 ziglang.org 公布的 minisign 公鑰 `RWSGOq2NVecA2UPNdBUZykf1CCb147pkmdtYxgb3Ti+JO/wCYvhbAb/U` 驗證（支援 prehashed `ED` 與舊式 `Ed`）。trusted comment 也要驗，而且裡面的 `file:` 必須是這個檔名。任何一步失敗就不安裝。
5. 解到 `~/.local/share/zig/zig-<arch>-<os>-<版本>/`，`~/.local/bin/zig` 是指向裡面 `zig` 的 symlink（標準函式庫在同一個目錄，zig 會照真實路徑找到）。`~/.bashrc`、`~/.zshrc` 會加 `~/.local/bin` 到 PATH。

zig 的 tarball 解開後有好幾萬個檔案、幾百 MiB，所以解壓上限調成 100000 個檔案、1 GiB。

## 解除安裝

- 套件安裝的：用套件管理器 remove，例如 Fedora `sudo dnf remove -y zig`。
- tarball 安裝的：刪 `~/.local/bin/zig` 和它指向的版本目錄，`~/.local/share/zig` 空了也一併刪除。zig 的快取（`~/.cache/zig`）不動。
