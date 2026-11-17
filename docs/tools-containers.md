# 容器分類：Docker 與 Podman

`tools-download` 與 `tools-uninstall` 的最後一個分類「容器」，順序是 1. docker、2. podman。預設不勾選，不屬於「基礎」，可以移除。

## Docker：依系統分開安裝

只用各發行版自己的套件庫，不加 docker-ce 第三方 repo（少一個要信任的來源，也不用管 repo 金鑰與發行版代號）。WSL 跟著裡面跑的發行版走（WSL Fedora 用 dnf 那一列，WSL Ubuntu 用 apt 那一列）。

| 系統 | 管理器 | 安裝指令 | 偵測 |
|------|--------|----------|------|
| Debian／Ubuntu（含 WSL） | apt | `sudo apt-get install -y --no-install-recommends docker.io` | `/usr/bin/docker` |
| Fedora（含 WSL Fedora） | dnf | `sudo dnf install -y moby-engine docker-compose` | `/usr/bin/docker` |
| CentOS 7／Amazon Linux 2 | yum | `sudo yum install -y docker` | `/usr/bin/docker` |
| Arch | pacman | `sudo pacman -S --needed --noconfirm docker docker-compose` | `/usr/bin/docker` |
| openSUSE | zypper | `sudo zypper --non-interactive install --no-recommends docker docker-compose` | `/usr/bin/docker` |
| Alpine | apk | `sudo apk add docker docker-cli-compose` | `/usr/bin/docker` |
| Void | xbps | `sudo xbps-install -y -S docker docker-compose` | `/usr/bin/docker` |
| macOS | brew | `brew install docker docker-compose` | `/opt/homebrew/bin/docker` 或 `/usr/local/bin/docker` |

- apt 只裝 `docker.io`：compose v2 在 Debian 與 Ubuntu 的套件名稱不同（`docker-compose`／`docker-compose-v2`），沒有共同名稱，需要時自己裝。
- yum 那一列給還在用 yum 的舊系統；RHEL 8 以後的 `docker` 其實是 podman-docker，建議改用 podman。
- macOS 的 brew 只裝 CLI 和 compose，沒有 daemon。要真的跑容器需要 Docker Desktop，或 `brew install colima && colima start`。

## Podman

每個 Linux 管理器都裝 `podman`（偵測 `/usr/bin/podman`），brew 裝 `podman`（偵測 `/opt/homebrew/bin/podman` 或 `/usr/local/bin/podman`）。免 daemon、可 rootless，不需要額外設定。mac 上要跑容器請自己執行 `podman machine init && podman machine start`。

## Docker 安裝後設定（只有 Linux）

Linux 上勾選 docker 時，安裝步驟最後多一個 `service docker` 步驟，同樣顯示在 TUI 進度頁、沿用 TUI 裡輸入過的 sudo，沒有 y 確認：

1. `sudo groupadd -f docker`
2. `sudo usermod -aG docker <使用者>`：使用者依序取 `Runner.User`、root 時的 `SUDO_USER`、目前使用者、`$USER`，名稱不合法就不執行。重新登入或 `newgrp docker` 後生效。
3. 有 systemd（`/run/systemd/system` 存在）時：`sudo systemctl enable --now docker.service`。

沒有 systemd 的 WSL 會跳過第 3 步，結果頁顯示提示：在 `/etc/wsl.conf` 加 `[boot]` `systemd=true`（再 `wsl --shutdown`），或手動執行 `sudo dockerd`。

docker 套件安裝失敗時這一步顯示 skipped。設定指令失敗時這一步標 failed、在 docker 的備註寫出失敗的指令，但 docker 本身仍算安裝成功。docker 已經裝好（satisfied）時也會跑這一步，指令都可以重複執行。

## 解除安裝

- 用套件管理器的 remove（不是 purge），例如 Fedora `sudo dnf remove -y moby-engine docker-compose`、Ubuntu `sudo apt-get remove -y docker.io`。
- 預設**不刪**映像檔、容器與 volume：`/var/lib/docker`、`/var/lib/containers`、`~/.local/share/containers` 都保留。要清掉請自己執行 `docker system prune -a --volumes`／`podman system reset`，或刪除上述目錄。
- docker 群組與使用者的群組成員也保留。
