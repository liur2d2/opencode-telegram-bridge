# opencode-telegram Bridge

[English](README.md) | **繁體中文**

Telegram ↔ OpenCode 雙向橋接服務。透過 Telegram 完全控制 OpenCode，支援 session 管理、agent 切換與互動式問答/權限提示。

## 功能特色

- ✅ **雙向同步**: 從 Telegram 發送訊息 → OpenCode TUI，並在 Telegram 接收回應
- ✅ **去重機制**: 智慧訊息去重，防止 Telegram 收到重複回應
- ✅ **Session 持久化**: 選定的 session 在服務重啟後保留
- ✅ **互動式命令**: 完整的 session 管理、agent 切換、模型選擇
- ✅ **背景服務**: 作為 macOS launchd daemon 執行，登入時自動啟動
- ✅ **即時更新**: OpenCode 回應透過 webhook 事件即時顯示在 Telegram

## 架構

本專案採用 **混合 plugin + service 架構**：

```
OpenCode Events → Plugin (TypeScript) → HTTP Webhook → Bridge Service (Go) → Telegram Bot
```

**元件說明:**
1. **OpenCode Plugin** (`~/.config/opencode/plugin/telegram-bridge/`)
   - TypeScript plugin，掛鉤 OpenCode 事件
   - 透過 HTTP webhook 傳送事件到 Bridge service
   - OpenCode 啟動時自動載入

2. **Bridge Service** (Go 二進位檔 + launchd)
   - 透過 HTTP webhook 接收來自 plugin 的事件
   - 管理 Telegram ↔ OpenCode 雙向通訊
   - 以背景服務方式執行，登入時自動啟動

**優勢:**
- ✅ 隨 OpenCode 自動啟動（無需手動管理服務）
- ✅ 終端機關閉後仍持續執行（launchd daemon）
- ✅ 保留完整雙向功能
- ✅ 職責清楚分離

## 安裝與部署

### 前置需求

1. **OpenCode 必須已安裝並設定完成**
   ```bash
   opencode serve --port 54321
   ```

2. **取得 Telegram Bot Token**
   - 在 Telegram 上傳訊息給 @BotFather
   - 建立新 bot 並取得 token

3. **取得你的 Telegram Chat ID**
   - 在 Telegram 上傳訊息給 @userinfobot
   - 記下你的 Chat ID

### 安裝步驟

1. **建置 Bridge Service:**
   ```bash
   cd ~/opencode-telegram
   go build -o opencode-telegram ./cmd
   ```

2. **建置 OpenCode Plugin:**
   ```bash
   cd ~/.config/opencode/plugin/telegram-bridge
   npm install
   npm run build
   ```

3. **設定 OpenCode 載入 plugin:**
   
   編輯 `~/.config/opencode/opencode.json`:
   ```json
   {
     "plugin": ["telegram-bridge"]
   }
   ```

4. **建立 plugin 設定檔:**
   
   建立 `~/.config/opencode/telegram-bridge.json`:
   ```json
   {
     "webhookUrl": "http://localhost:8888/webhook",
     "enabled": true
   }
   ```

5. **設定 launchd service:**
   
   編輯 `~/Library/LaunchAgents/com.opencode.telegram.bridge.plist`:
   - 將 `TELEGRAM_BOT_TOKEN` 替換為你的 bot token
   - 將 `TELEGRAM_CHAT_ID` 替換為你的 chat ID
   - 若需要可調整路徑

6. **載入服務:**
   ```bash
   launchctl load ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
   ```

7. **驗證服務已執行:**
   ```bash
   launchctl list | grep telegram
   curl http://localhost:8888/health
   ```

### 解除安裝

```bash
launchctl unload ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
rm ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
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

launchd service (`~/Library/LaunchAgents/com.opencode.telegram.bridge.plist`) 設定:

**必需:**
- `TELEGRAM_BOT_TOKEN`: 你的 Telegram bot token（從 @BotFather 取得）
- `TELEGRAM_CHAT_ID`: 你的 chat ID

**選填:**
- `OPENCODE_BASE_URL`: OpenCode 伺服器 URL（預設：`http://localhost:54321`）
- `OPENCODE_DIRECTORY`: OpenCode 設定目錄（預設：`~/.config/opencode`）
- `USE_PLUGIN_MODE`: 啟用 plugin 模式（預設：`true`）
- `PLUGIN_WEBHOOK_PORT`: Plugin webhook port（預設：`8888`）
- `HEALTH_PORT`: Health/metrics endpoint port（預設：`8080`）
- `TELEGRAM_STATE_FILE`: Session 狀態持久化檔案（預設：`~/.opencode-telegram-state`）
- `TELEGRAM_OFFSET_FILE`: Telegram update offset 檔案（預設：`~/.opencode-telegram-offset`）

### LaunchAgent 設定

plist 設定了:

- **RunAtLoad**: 登入時自動啟動
- **KeepAlive**: 當機時自動重啟
- **StandardOutPath**: 日誌輸出至 `~/.local/var/log/opencode-telegram.log`
- **StandardErrorPath**: 錯誤輸出至 `~/.local/var/log/opencode-telegram-error.log`

### 日誌檔案

即時查看日誌:
```bash
tail -f ~/.local/var/log/opencode-telegram.log
tail -f ~/.local/var/log/opencode-telegram-error.log
```

## 使用方式

啟動後，透過 Telegram 控制 OpenCode：

### 指令

- `/help` — 顯示所有可用指令
- `/status` — 顯示目前 session、agent、模型、目錄與 OpenCode 健康狀態

### Session 管理
- `/new [title]` — 建立新 session
- `/sessions` — 列出主要 sessions（表格檢視，最多 15 個）
- `/selectsession` — 互動式 session 選擇器（含分頁）
- `/deletesessions` — 刪除 sessions（互動式選擇）
- `/abort` — 中止目前請求

**注意**: 目前選定的 session 會透過 `~/.opencode-telegram-state` 在服務重啟後保留。

### Agent 與 Model 選擇
- `/route [agent]` — 設定 agent 路由（或透過互動式選單顯示目前 agent）
- `/model` — 選擇 AI 模型（互動式選單，含分頁）

### 互動式功能
- 問題以 Inline Keyboard 顯示 → 點擊回答
- 權限以 Inline Keyboard 顯示 → 點擊 Allow/Reject/Always Allow
- 訊息上的 Reaction（👍👎）會轉發給 AI
- Sticker 會被描述後傳送給 AI

## 開發

### 從原始碼建置

**Bridge Service:**
```bash
cd ~/opencode-telegram
go build -o opencode-telegram ./cmd
```

**OpenCode Plugin:**
```bash
cd ~/.config/opencode/plugin/telegram-bridge
npm run build
```

### 開發模式執行

**舊版 SSE 模式（不使用 plugin）:**
```bash
export USE_PLUGIN_MODE=false
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_CHAT_ID="your-chat-id"
./opencode-telegram
```

**Plugin 模式（推薦）:**
1. 確保 plugin 已建置並在 `opencode.json` 中註冊
2. 以 `USE_PLUGIN_MODE=true` 啟動 bridge service
3. 以 `opencode serve` 啟動 OpenCode

### 測試 Webhook

手動測試 webhook endpoint:
```bash
curl -X POST http://localhost:8888/webhook \
  -H "Content-Type: application/json" \
  -d '{"type":"session.created","data":{"sessionId":"test","directory":"/test"},"timestamp":1707378800000}'
```

健康檢查:
```bash
curl http://localhost:8888/health
curl http://localhost:8080/metrics
```

## 技術架構

### 元件說明

**OpenCode Plugin** (`~/.config/opencode/plugin/telegram-bridge/`):
- TypeScript plugin 使用 `@opencode-ai/plugin` SDK
- 掛鉤事件: `session.created`, `message.updated`, `session.idle`
- 傳送 HTTP POST 到 webhook server
- 設定檔: `~/.config/opencode/telegram-bridge.json`

**Webhook Server** (`internal/webhook/server.go`):
- 接收來自 plugin 的 HTTP webhooks
- 轉換為內部 SSE Event 格式
- 轉發到 Bridge event handler
- Endpoints: `/webhook`, `/health`

**Bridge Service** (`internal/bridge/bridge.go`):
- 協調 Telegram ↔ OpenCode 雙向通訊
- Debouncing、訊息串流、權限/問題處理
- Goroutine-safe 狀態管理

**Telegram Bot** (`internal/telegram/bot.go`):
- `go-telegram/bot` library 的封裝
- 訊息格式化（HTML）、inline keyboards
- Polling 模式（不需要 webhook）

**State Management** (`internal/state/`):
- Session/agent 狀態追蹤
- Session 狀態持久化（跨重啟保留，`state.go`）
- Telegram update offset 追蹤（`offset.go`）
- Callback ID registry（inline keyboards 的短 ID）
- Goroutine-safe with sync.Map

**去重機制** (`internal/bridge/bridge.go`):
- 基於 MessageID 的去重機制，防止 Telegram 收到重複回應
- **精準訊息查詢**: 使用 `GetMessage(sessionID, messageID)` API 取得特定完成訊息
- **事件驅動的 MessageID 提取**: `handleMessageUpdated` 直接從事件中提取 messageID
- `message.updated` 與 `session.idle` 事件共用去重快取，使用 `msg:{messageID}` key 格式
- 每個唯一訊息 60 秒 TTL，自動清理
- 原子性 `LoadOrStore` 操作確保 goroutine 安全
- 自動過濾: 跳過 user 訊息（role != "assistant"）與空內容

### 事件流程

```
使用者在 Telegram 傳送訊息
  ↓
Telegram Bot 接收 update
  ↓
Bridge.HandleUserMessage()
  ↓
OpenCode Client.TriggerPrompt()
  ↓
OpenCode 處理請求
  ↓
Plugin 接收事件 (session.idle, message.updated)
  ↓
Plugin 傳送 HTTP POST 到 Webhook Server
  ↓
Webhook Server 轉換為 SSE Event
  ↓
Bridge.HandleSSEEvent()
  ↓
Telegram Bot 傳送回應
```

## 疑難排解

### 常見問題

**TUI 未顯示 Telegram 訊息:**
- 訊息已成功傳送到 OpenCode API 並儲存在 session 中
- 這是 OpenCode TUI 的已知限制 - 透過 API 收到訊息時不會自動重新整理
- 驗證訊息在 session 中: `curl http://localhost:54321/session/<session-id>/message?limit=5`
- 解決方法: 在 TUI 發送任何訊息以觸發 UI 重新整理

**Telegram 收到重複訊息:**
- 已透過基於 messageID 的精準查詢修正（v1.0+）
- 根本原因: 舊版本查詢最新訊息，在新請求到達時可能返回舊訊息
- 解決方案: 從 `message.updated` 事件提取 messageID → 透過 `/session/{id}/message/{messageID}` 查詢特定訊息 → 使用 `msg:{messageID}` cache key 去重
- 每條 OpenCode 回應只會傳送到 Telegram 一次
- 去重快取: 60 秒 TTL，自動清理

**重啟後 session 遺失:**
- 確保 launchd plist 中已設定 `TELEGRAM_STATE_FILE` 環境變數
- 預設位置: `~/.opencode-telegram-state`
- 檢查檔案存在且包含有效的 session ID

### 服務問題

**服務無法啟動:**
```bash
launchctl list | grep telegram
tail -f ~/.local/var/log/opencode-telegram-error.log
```

**Webhook server 未監聽:**
```bash
lsof -i :8888
curl http://localhost:8888/health
```

**Plugin 未載入:**
```bash
cat ~/.config/opencode/opencode.json
ls -la ~/.config/opencode/plugin/telegram-bridge/dist/
```

### 連線問題

**OpenCode 無法連線:**
```bash
curl http://localhost:54321/health
ps aux | grep "opencode serve"
lsof -i :54321
```

**Telegram bot 衝突:**
- 錯誤: "terminated by other getUpdates request"
- 解決方式: 同一時間只能有一個 bot 實例進行 polling
  ```bash
  killall opencode-telegram
  launchctl unload ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
  launchctl load ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
  ```

### Port 衝突

**Port 8080 或 8888 已被使用:**
```bash
lsof -i :8080
lsof -i :8888
kill <PID>
```

或在 plist 中變更 port:
```xml
<key>PLUGIN_WEBHOOK_PORT</key>
<string>9999</string>
<key>HEALTH_PORT</key>
<string>9090</string>
```

### Plugin 問題

**Plugin 未傳送 webhooks:**
1. 檢查 OpenCode 日誌是否有 plugin 錯誤
2. 驗證 `~/.config/opencode/telegram-bridge.json` 中的 webhook URL
3. 手動測試 webhook:
   ```bash
   curl -X POST http://localhost:8888/webhook \
     -H "Content-Type: application/json" \
     -d '{"type":"session.idle","data":{"sessionId":"test"},"timestamp":1707378800000}'
   ```

**Plugin 建置錯誤:**
```bash
cd ~/.config/opencode/plugin/telegram-bridge
rm -rf node_modules dist
npm install
npm run build
```
