# opencode-telegram Bridge

[English](README.md) | **繁體中文**

Telegram ↔ OpenCode 雙向橋接服務。透過 Telegram 完全控制 OpenCode，支援 session 管理、agent 切換與互動式問答/權限提示。

## 安裝與部署

### 快速開始 (macOS LaunchAgent)

1. **編輯 plist 檔案，填入你的憑證：**
   ```bash
   nano configs/com.opencode.telegram-bridge.plist
   ```

   替換以下佔位符：
   - `YOUR_USERNAME` → 你的 macOS 使用者名稱（例如：`john`）
   - `YOUR_BOT_TOKEN_HERE` → 從 @BotFather 取得的 Telegram bot token
   - `YOUR_CHAT_ID_HERE` → 你的 Telegram chat ID

2. **執行安裝腳本：**
   ```bash
   ./scripts/install.sh
   ```

   這會：
   - 建立日誌目錄：`~/Library/Logs/opencode-telegram/`
   - 安裝 plist 到：`~/Library/LaunchAgents/com.opencode.telegram-bridge.plist`
   - 透過 `launchctl` 載入服務

3. **驗證服務已執行：**
   ```bash
   launchctl list | grep com.opencode.telegram-bridge
   ```

4. **查看日誌：**
   ```bash
   tail -f ~/Library/Logs/opencode-telegram/stdout.log
   tail -f ~/Library/Logs/opencode-telegram/stderr.log
   ```

### 手動安裝（不使用腳本）

1. 複製 plist：
   ```bash
   cp configs/com.opencode.telegram-bridge.plist ~/Library/LaunchAgents/
   ```

2. 編輯並填入憑證：
   ```bash
   nano ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
   ```

3. 載入服務：
   ```bash
   launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
   ```

### 解除安裝

```bash
launchctl unload ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
rm ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
```

## 設定

### 環境變數

**必需：**
- `TELEGRAM_BOT_TOKEN`: 你的 Telegram bot token（從 @BotFather 取得）
- `TELEGRAM_CHAT_ID`: 你的 chat ID（透過對 bot 傳送 `/chatid` 指令取得）

**選填：**
- `OPENCODE_BASE_URL`: OpenCode 伺服器 URL（預設：`http://localhost:54321`）
- `OPENCODE_DIRECTORY`: OpenCode 設定檔目錄（預設：`~/.config/opencode`）
- `OPENCODE_SERVER_PASSWORD`: 如需驗證則填入伺服器密碼

### LaunchAgent Plist 詳細說明

plist 檔案（`configs/com.opencode.telegram-bridge.plist`）設定了：

- **Program**: 二進位檔案路徑
- **WorkingDirectory**: 專案根目錄
- **EnvironmentVariables**: 所有設定透過環境變數（程式碼中不含敏感資訊）
- **RunAtLoad**: 登入時自動啟動
- **KeepAlive**: 當機時自動重啟
  - `SuccessfulExit: false` → 即使成功結束也重啟（daemon 行為）
- **StandardOutPath**: 日誌輸出至 `~/Library/Logs/opencode-telegram/stdout.log`
- **StandardErrorPath**: 錯誤輸出至 `~/Library/Logs/opencode-telegram/stderr.log`

### 日誌檔案

日誌寫入：`~/Library/Logs/opencode-telegram/`

- `stdout.log` — 一般日誌
- `stderr.log` — 錯誤日誌

即時查看日誌：
```bash
tail -f ~/Library/Logs/opencode-telegram/stdout.log
tail -f ~/Library/Logs/opencode-telegram/stderr.log
```

## 從原始碼建置

```bash
go build -o opencode-telegram ./cmd
```

## 驗證清單

✅ Plist 語法正確（`plutil -lint`）  
✅ 使用佔位符供使用者填入數值  
✅ KeepAlive 設定為自動重啟  
✅ 日誌導向 `~/Library/Logs/opencode-telegram/`  
✅ 安裝腳本處理路徑展開（`$HOME`）  
✅ 服務能用 `launchctl` 順利載入/卸載

## 使用方式

啟動後，透過 Telegram 控制 OpenCode：

### 指令

- `/help` — 顯示所有可用指令
- `/status` — 顯示目前 session、agent、模型、目錄與 OpenCode 健康狀態

### Session 管理
- `/new [title]` — 建立新 session
- `/sessions` — 列出主要 sessions（表格檢視，最多 15 個）
- `/selectsession` — 互動式 session 選擇器（含分頁）
- `/abort` — 中止目前請求

### Agent 與 Model 選擇
- `/route [agent]` — 設定 agent 路由（或透過互動式選單顯示目前 agent）
- `/model` — 選擇 AI 模型（互動式選單，含分頁）

### 互動式功能
- 問題以 Inline Keyboard 顯示 → 點擊回答
- 權限以 Inline Keyboard 顯示 → 點擊 Allow/Reject/Always Allow
- 訊息上的 Reaction（👍👎）會轉發給 AI
- Sticker 會被描述後傳送給 AI

## 架構

- **Bridge**: 協調 Telegram ↔ OpenCode 雙向通訊
- **OpenCode Client**: OpenCode API 的 HTTP 封裝器 + SSE 事件消費者
- **Telegram Bot**: `go-telegram/bot` 的封裝，具訊息格式化與鍵盤功能
- **State Management**: Goroutine-safe 的 session/agent 狀態 + callback ID 登錄
- **Handlers**: 指令、agent、問題、權限、訊息轉發處理器

## 疑難排解

**服務無法啟動：**
```bash
launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
launchctl list | grep com.opencode.telegram-bridge
cat ~/Library/Logs/opencode-telegram/stderr.log
```

**缺少憑證：**
- 檢查 plist 中的 TELEGRAM_BOT_TOKEN 與 TELEGRAM_CHAT_ID
- 查看日誌中的變數缺失錯誤

**OpenCode 無法連線：**
- 檢查 `OPENCODE_BASE_URL` 是否正確（預設：http://localhost:54321）
- 確認 OpenCode 正在執行：`ps aux | grep "opencode serve"`

**服務持續重啟：**
- 查看錯誤日誌：`tail -f ~/Library/Logs/opencode-telegram/stderr.log`
- 確認 plist 中的所有環境變數都已正確設定
