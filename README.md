# opencode-telegram Bridge

Telegram ↔ OpenCode bidirectional bridge service. Control OpenCode entirely via Telegram with session management, agent switching, and interactive question/permission prompts.

## Installation & Deployment

### Quick Start (macOS LaunchAgent)

1. **Edit the plist with your credentials:**
   ```bash
   nano configs/com.opencode.telegram-bridge.plist
   ```

   Replace placeholders:
   - `YOUR_USERNAME` → your macOS username (e.g., `john`)
   - `YOUR_BOT_TOKEN_HERE` → Telegram bot token from @BotFather
   - `YOUR_CHAT_ID_HERE` → your Telegram chat ID

2. **Run the installer script:**
   ```bash
   ./scripts/install.sh
   ```

   This will:
   - Create log directory: `~/Library/Logs/opencode-telegram/`
   - Install plist to: `~/Library/LaunchAgents/com.opencode.telegram-bridge.plist`
   - Load the service via `launchctl`

3. **Verify service is running:**
   ```bash
   launchctl list | grep com.opencode.telegram-bridge
   ```

4. **Check logs:**
   ```bash
   tail -f ~/Library/Logs/opencode-telegram/stdout.log
   tail -f ~/Library/Logs/opencode-telegram/stderr.log
   ```

### Manual Installation (without script)

1. Copy plist:
   ```bash
   cp configs/com.opencode.telegram-bridge.plist ~/Library/LaunchAgents/
   ```

2. Edit it with your credentials:
   ```bash
   nano ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
   ```

3. Load service:
   ```bash
   launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
   ```

### Uninstall

```bash
launchctl unload ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
rm ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
```

## Configuration

### Environment Variables

**Required:**
- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token (from @BotFather)
- `TELEGRAM_CHAT_ID`: Your chat ID (get via `/chatid` command to bot)

**Optional:**
- `OPENCODE_BASE_URL`: OpenCode server URL (default: `http://localhost:54321`)
- `OPENCODE_DIRECTORY`: OpenCode config directory (default: `~/.config/opencode`)
- `OPENCODE_SERVER_PASSWORD`: Server password if authentication required

### LaunchAgent Plist Details

The plist (`configs/com.opencode.telegram-bridge.plist`) configures:

- **Program**: Binary path
- **WorkingDirectory**: Project root directory
- **EnvironmentVariables**: All configuration via env vars (no secrets in code)
- **RunAtLoad**: Auto-start on login
- **KeepAlive**: Auto-restart on crash
  - `SuccessfulExit: false` → restart even on successful exit (desired for daemon)
- **StandardOutPath**: Logs to `~/Library/Logs/opencode-telegram/stdout.log`
- **StandardErrorPath**: Logs to `~/Library/Logs/opencode-telegram/stderr.log`

### Log Files

Logs are written to: `~/Library/Logs/opencode-telegram/`

- `stdout.log` — Regular logging
- `stderr.log` — Error logging

View live logs:
```bash
tail -f ~/Library/Logs/opencode-telegram/stdout.log
tail -f ~/Library/Logs/opencode-telegram/stderr.log
```

## Building from Source

```bash
go build -o opencode-telegram ./cmd
```

## Verification Checklist

✅ Plist syntax valid (`plutil -lint`)
✅ Placeholders for user-specific values
✅ KeepAlive configured for auto-restart
✅ Logs directed to `~/Library/Logs/opencode-telegram/`
✅ Installation script handles path expansion (`$HOME`)
✅ Service loads/unloads cleanly with `launchctl`

## Usage

Once running, control OpenCode via Telegram:

### Session Management
- `/newsession [title]` — Create new session
- `/sessions` — List all sessions
- `/session {id}` — Switch to specific session
- `/abort` — Abort current session
- `/status` — Show current session + server health

### Agent Switching
- `/switch [agent]` — Switch OHO agent (or show available agents)

### Commands
- `/help` — Show all available commands

### Interactive Prompts
- Questions appear as Inline Keyboards → tap to answer
- Permissions appear as Inline Keyboards → tap Allow/Reject

## Architecture

- **Bridge**: Orchestrates Telegram ↔ OpenCode bidirectional communication
- **OpenCode Client**: HTTP wrapper for OpenCode API + SSE event consumer
- **Telegram Bot**: Wrapper around `go-telegram/bot` with message formatting and keyboards
- **State Management**: Goroutine-safe session/agent state + callback ID registry
- **Handlers**: Commands, agents, questions, permissions, message forwarding

## Troubleshooting

**Service not starting:**
```bash
launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
launchctl list | grep com.opencode.telegram-bridge
cat ~/Library/Logs/opencode-telegram/stderr.log
```

**Missing credentials:**
- Verify TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID in plist
- Check logs for specific missing variable errors

**OpenCode unreachable:**
- Verify `OPENCODE_BASE_URL` is correct (default: http://localhost:54321)
- Check OpenCode is running: `ps aux | grep "opencode serve"`

**Service keeps restarting:**
- Check error logs: `tail -f ~/Library/Logs/opencode-telegram/stderr.log`
- Verify all environment variables are set correctly in plist
