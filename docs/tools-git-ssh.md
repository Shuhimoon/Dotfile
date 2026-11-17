# GitHub SSH 鑰匙設定

安裝流程跑完、而且 git 的結果是「完成／已安裝」（ok、satisfied、shadowed）時，結果頁底欄改成「任意鍵 設定 GitHub SSH 鑰匙」，按任意鍵進入設定頁；其他情況（git 失敗、沒勾 git、只顯示計畫、移除流程）照舊回主頁。git 在「基礎」類而且預設勾選，所以已經裝好 git 的機器跑一次安裝也會出現這頁，按 `esc` 就略過。

## 鑰匙放哪

| 系統 | 列出／儲存的目錄 | config 裡的 IdentityFile |
|------|------------------|--------------------------|
| Mac／Linux | `~/.ssh/keys/` | `~/.ssh/keys/<鑰匙名稱>` |
| Windows（for_windows 分支） | `%USERPROFILE%\.ssh\` | `~/.ssh/<鑰匙名稱>` |

## 流程

1. **選鑰匙**：列出目錄裡的私鑰。只看一般檔案，內容開頭是 `-----BEGIN … PRIVATE KEY-----`（含 `OPENSSH PRIVATE KEY`、`RSA PRIVATE KEY` 等）才算；`*.pub`、`known_hosts*`、`authorized_keys*`、`config*`、`*.bak*`、隱藏檔都略過。`↑/↓` 選，`enter` 使用，最後一項「＋ 貼上新的私鑰…」進入新增頁，`esc` 略過整個設定。如果 config 已經有我們的區塊，頁面會顯示目前設定。
2. **新增鑰匙**（目錄裡沒有私鑰時直接進這頁）：
   - 私鑰欄：直接貼上（支援 bracketed paste；沒有的話每一行的換行會當成 enter，也一樣）。畫面只顯示「已貼上 N 行、M 字元」和第一行標頭，**不顯示鑰匙內容**。`ctrl+u` 清除。
   - `tab` 切到名稱欄。名稱只能用英數字和 `. _ -`，開頭必須是英數字，最多 64 字，不能有斜線，不能是 `*.pub`、`config`、`known_hosts` 這類名稱。
   - 名稱欄按 `enter`（或任一欄按 `ctrl+s`）儲存。同名檔案已存在就拒絕並請你換名稱，**不覆蓋**（不用 y 確認）。
   - 儲存前會把 CRLF 換成 LF、去掉前後空白、補上結尾換行，再用 `golang.org/x/crypto/ssh` 的 `ParseRawPrivateKey` 檢查；有密碼保護的鑰匙照原樣接受。錯誤訊息不會帶出鑰匙內容。
   - 檔案權限 600，`~/.ssh` 與 `~/.ssh/keys` 是 700。鑰匙文字只在貼上時留在記憶體，儲存或離開時清掉，不會寫進紀錄。
   - `esc`：有現成鑰匙時回到選擇頁，否則略過設定。
3. **寫 `~/.ssh/config`**（權限 600），內容是受管理的區塊：

   ```
   # >>> dotfiles:github >>>
   Host github.com
     HostName github.com
     User git
     IdentityFile ~/.ssh/keys/<鑰匙名稱>
   # <<< dotfiles:github <<<
   ```

   - 每次都先移除舊的受管理區塊，再把新區塊放在檔案**最上面**，其餘內容原樣接在後面，所以重跑不會重複（內容相同就不寫檔）。
   - 第一次加區塊前，如果原本有 config，就先備份成 `config.bak.<UTC 時間>`。
   - 如果 config 本來就有自己寫的 `Host github.com`，不會去改它，只在結果顯示警告。新的區塊在最上面，ssh 對每個選項採用第一個找到的值，所以會優先用我們的 IdentityFile。
4. 完成頁列出做了什麼，以及測試指令 `ssh -T git@github.com`，任意鍵回主頁。

## 移除

移除 git 時（Mac／Linux 的 git 在「基礎」類不能移除，所以實際上只有 Windows 會用到）會多一個 `ssh-config git` 步驟：只刪 config 裡的受管理區塊，鑰匙檔保留。`d` 計畫頁也會列出這一步。
