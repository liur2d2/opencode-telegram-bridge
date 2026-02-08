# opencode-telegram Bridge

**English** | [繁體中文](README.zh-TW.md)

Telegram ↔ OpenCode bidirectional bridge service. Control OpenCode entirely via Telegram with session management, agent switching, and interactive question/permission prompts.

## Features

- ✅ **Bidirectional Sync**: Send messages from Telegram → OpenCode TUI and receive responses in Telegram
- ✅ **Deduplication**: Intelligent message deduplication prevents duplicate responses in Telegram
- ✅ **Session Persistence**: Selected session persists across service restarts
- ✅ **Interactive Commands**: Full session management, agent switching, and model selection
- ✅ **Background Service**: Runs as macOS launchd daemon, auto-starts on login
- ✅ **Real-time Updates**: OpenCode responses appear in Telegram via webhook events

## Architecture

This project uses a **hybrid plugin + service architecture**:

```
OpenCode Events → Plugin (TypeScript) → HTTP Webhook → Bridge Service (Go) → Telegram Bot
```

**Components:**
1. **OpenCode Plugin** (`~/.config/opencode/plugin/telegram-bridge/`)
   - TypeScript plugin that hooks into OpenCode events
   - Sends events via HTTP webhooks to the Bridge service
   - Auto-loaded when OpenCode starts

2. **Bridge Service** (Go binary with launchd)
   - Receives events from the plugin via HTTP webhook
   - Manages bidirectional Telegram ↔ OpenCode communication
   - Runs as background service, auto-starts on login

**Benefits:**
- ✅ Auto-starts with OpenCode (no manual service management)
- ✅ Survives terminal closures (runs as launchd daemon)
- ✅ Maintains full bidirectional features
- ✅ Clean separation of concerns

## Installation & Deployment

### Prerequisites

1. **OpenCode must be installed and configured**
   ```bash
   opencode serve --port 54321
   ```

2. **Get Telegram Bot Token**
   - Message @BotFather on Telegram
   - Create new bot and get token

3. **Get your Telegram Chat ID**
   - Message @userinfobot on Telegram
   - Note your Chat ID

### Installation Steps

1. **Build the Bridge Service:**
   ```bash
   cd ~/opencode-telegram
   go build -o opencode-telegram ./cmd
   ```

2. **Build the OpenCode Plugin:**
   ```bash
   cd ~/.config/opencode/plugin/telegram-bridge
   npm install
   npm run build
   ```

3. **Configure OpenCode to load the plugin:**
   
   Edit `~/.config/opencode/opencode.json`:
   ```json
   {
     "plugin": ["telegram-bridge"]
   }
   ```

4. **Create plugin configuration:**
   
   Create `~/.config/opencode/telegram-bridge.json`:
   ```json
   {
     "webhookUrl": "http://localhost:8888/webhook",
     "enabled": true
   }
   ```

5. **Configure launchd service:**
   
   Edit `~/Library/LaunchAgents/com.opencode.telegram.bridge.plist`:
   - Replace `TELEGRAM_BOT_TOKEN` with your bot token
   - Replace `TELEGRAM_CHAT_ID` with your chat ID
   - Adjust paths if needed

6. **Load the service:**
   ```bash
   launchctl load ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
   ```

7. **Verify service is running:**
   ```bash
   launchctl list | grep telegram
   curl http://localhost:8888/health
   ```

### Uninstall

```bash
launchctl unload ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
rm ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
```

## Configuration

### Environment Variables

The launchd service (`~/Library/LaunchAgents/com.opencode.telegram.bridge.plist`) configures:

**Required:**
- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token (from @BotFather)
- `TELEGRAM_CHAT_ID`: Your chat ID

**Optional:**
- `OPENCODE_BASE_URL`: OpenCode server URL (default: `http://localhost:54321`)
- `OPENCODE_DIRECTORY`: OpenCode config directory (default: `~/.config/opencode`)
- `USE_PLUGIN_MODE`: Enable plugin mode (default: `true`)
- `PLUGIN_WEBHOOK_PORT`: Plugin webhook port (default: `8888`)
- `HEALTH_PORT`: Health/metrics endpoint port (default: `8080`)
- `TELEGRAM_STATE_FILE`: Session state persistence file (default: `~/.opencode-telegram-state`)
- `TELEGRAM_OFFSET_FILE`: Telegram update offset file (default: `~/.opencode-telegram-offset`)

### LaunchAgent Configuration

The plist configures:

- **RunAtLoad**: Auto-start on login
- **KeepAlive**: Auto-restart on crash
- **StandardOutPath**: Logs to `~/.local/var/log/opencode-telegram.log`
- **StandardErrorPath**: Logs to `~/.local/var/log/opencode-telegram-error.log`

### Log Files

View live logs:
```bash
tail -f ~/.local/var/log/opencode-telegram.log
tail -f ~/.local/var/log/opencode-telegram-error.log
```

## Development

### Building from Source

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

### Running in Development Mode

**Legacy SSE Mode (without plugin):**
```bash
export USE_PLUGIN_MODE=false
export TELEGRAM_BOT_TOKEN="your-token"
export TELEGRAM_CHAT_ID="your-chat-id"
./opencode-telegram
```

**Plugin Mode (recommended):**
1. Ensure plugin is built and registered in `opencode.json`
2. Start bridge service with `USE_PLUGIN_MODE=true`
3. Start OpenCode with `opencode serve`

### Testing Webhook

Test the webhook endpoint manually:
```bash
curl -X POST http://localhost:8888/webhook \
  -H "Content-Type: application/json" \
  -d '{"type":"session.created","data":{"sessionId":"test","directory":"/test"},"timestamp":1707378800000}'
```

Health check:
```bash
curl http://localhost:8888/health
curl http://localhost:8080/metrics
```

## Usage

Once running, control OpenCode via Telegram:

### Commands

- `/help` — Show all available commands
- `/status` — Show current session, agent, model, directory, and OpenCode health

### Session Management
- `/new [title]` — Create new session
- `/sessions` — List primary sessions (table view, up to 15)
- `/selectsession` — Interactive session selector with pagination
- `/deletesessions` — Delete sessions with interactive selection
- `/abort` — Abort current request

**Note**: Currently selected session persists across service restarts via `~/.opencode-telegram-state`.

### Agent & Model Selection
- `/route [agent]` — Set agent routing (or show current agent with interactive menu)
- `/model` — Select AI model (interactive menu with pagination)

### Interactive Prompts
- Questions appear as Inline Keyboards → tap to answer
- Permissions appear as Inline Keyboards → tap Allow/Reject/Always Allow
- Reactions (👍👎) on messages are forwarded to AI
- Stickers are described and sent to AI

## Technical Architecture

### Components

**OpenCode Plugin** (`~/.config/opencode/plugin/telegram-bridge/`):
- TypeScript plugin using `@opencode-ai/plugin` SDK
- Hooks: `session.created`, `message.updated`, `session.idle`
- Sends HTTP POST to webhook server
- Configuration: `~/.config/opencode/telegram-bridge.json`

**Webhook Server** (`internal/webhook/server.go`):
- Receives HTTP webhooks from plugin
- Converts to internal SSE Event format
- Forwards to Bridge event handler
- Endpoints: `/webhook`, `/health`

**Bridge Service** (`internal/bridge/bridge.go`):
- Orchestrates Telegram ↔ OpenCode bidirectional communication
- Debouncing, message streaming, permission/question handling
- Goroutine-safe state management

**Telegram Bot** (`internal/telegram/bot.go`):
- Wrapper around `go-telegram/bot` library
- Message formatting (HTML), inline keyboards
- Polling mode (no webhook required)

**State Management** (`internal/state/`):
- Session/agent state tracking
- Session state persistence across restarts (`state.go`)
- Telegram update offset tracking (`offset.go`)
- Callback ID registry (short IDs for inline keyboards)
- Goroutine-safe with sync.Map

**Deduplication** (`internal/bridge/bridge.go`):
- MessageID-based deduplication prevents duplicate Telegram responses
- Unified cache for `message.updated` and `session.idle` events
- 60-second TTL per unique message
- Atomic `LoadOrStore` operations for goroutine safety

### Event Flow

```
User sends message in Telegram
  ↓
Telegram Bot receives update
  ↓
Bridge.HandleUserMessage()
  ↓
OpenCode Client.TriggerPrompt()
  ↓
OpenCode processes request
  ↓
Plugin receives event (session.idle, message.updated)
  ↓
Plugin sends HTTP POST to Webhook Server
  ↓
Webhook Server converts to SSE Event
  ↓
Bridge.HandleSSEEvent()
  ↓
Telegram Bot sends response
```

## Troubleshooting

### Common Issues

**TUI not showing Telegram messages:**
- Messages ARE successfully sent to OpenCode API and stored in the session
- This is a known OpenCode TUI limitation - it doesn't auto-refresh when messages arrive via API
- Verify messages are in session: `curl http://localhost:54321/session/<session-id>/message?limit=5`
- Workaround: Send any message from TUI to trigger UI refresh

**Duplicate messages in Telegram:**
- Fixed via messageID-based deduplication (v1.0+)
- Each OpenCode response is sent exactly once to Telegram
- Deduplication cache: 60 seconds per unique message

**Session lost after restart:**
- Ensure `TELEGRAM_STATE_FILE` environment variable is set in launchd plist
- Default location: `~/.opencode-telegram-state`
- Check file exists and contains valid session ID

### Service Issues

**Service not starting:**
```bash
launchctl list | grep telegram
tail -f ~/.local/var/log/opencode-telegram-error.log
```

**Webhook server not listening:**
```bash
lsof -i :8888
curl http://localhost:8888/health
```

**Plugin not loaded:**
```bash
cat ~/.config/opencode/opencode.json
ls -la ~/.config/opencode/plugin/telegram-bridge/dist/
```

### Connection Issues

**OpenCode unreachable:**
```bash
curl http://localhost:54321/health
ps aux | grep "opencode serve"
lsof -i :54321
```

**Telegram bot conflicts:**
- Error: "terminated by other getUpdates request"
- Solution: Only one bot instance can poll at a time
  ```bash
  killall opencode-telegram
  launchctl unload ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
  launchctl load ~/Library/LaunchAgents/com.opencode.telegram.bridge.plist
  ```

### Port Conflicts

**Port 8080 or 8888 already in use:**
```bash
lsof -i :8080
lsof -i :8888
kill <PID>
```

Or change ports in plist:
```xml
<key>PLUGIN_WEBHOOK_PORT</key>
<string>9999</string>
<key>HEALTH_PORT</key>
<string>9090</string>
```

### Plugin Issues

**Plugin not sending webhooks:**
1. Check OpenCode logs for plugin errors
2. Verify webhook URL in `~/.config/opencode/telegram-bridge.json`
3. Test webhook manually:
   ```bash
   curl -X POST http://localhost:8888/webhook \
     -H "Content-Type: application/json" \
     -d '{"type":"session.idle","data":{"sessionId":"test"},"timestamp":1707378800000}'
   ```

**Plugin build errors:**
```bash
cd ~/.config/opencode/plugin/telegram-bridge
rm -rf node_modules dist
npm install
npm run build
```
