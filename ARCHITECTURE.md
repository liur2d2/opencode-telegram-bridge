# OpenCode Telegram Bot Architecture

## Communication Flow

### Message Processing Flow

```
User Message → Telegram Bot → Bridge → OpenCode API
                                ↓
                         SSE Consumer ← OpenCode Events
                                ↓
                         Bridge Handler → Telegram Bot → User
```

### Key Components

#### 1. Bridge (`internal/bridge/bridge.go`)
- Coordinates between Telegram and OpenCode
- Manages session state and message debouncing
- Handles user interactions (messages, callbacks, photos)

#### 2. SSE Consumer (`internal/opencode/sse.go`)
- Maintains persistent connection to OpenCode `/event` endpoint
- Receives real-time events (message updates, questions, permissions)
- Auto-reconnects with exponential backoff

#### 3. OpenCode Client (`internal/opencode/client.go`)
- HTTP client for OpenCode REST API
- **Timeout: 60 seconds** (sufficient for triggering requests)
- Does NOT wait for completion - uses SSE for responses

## Response Mechanism (SSE Push Model)

### Why SSE Instead of Synchronous HTTP?

**Previous (Problematic) Architecture:**
```
User → SendPrompt → Wait 30 minutes → Response → Telegram
                    ↓
                (Timeout Risk)
```

**Current (Optimized) Architecture:**
```
User → SendPrompt (trigger only, returns immediately)
         ↓
    OpenCode processes asynchronously
         ↓
    SSE: message.updated event
         ↓
    handleMessageUpdated → Telegram
```

### Benefits

1. **No Timeout Issues**: HTTP request only triggers processing, doesn't wait
2. **Real-time Updates**: SSE provides streaming updates as AI generates response
3. **Better UX**: Typing indicators work properly during long operations
4. **Scalable**: Bot can handle multiple concurrent sessions

### Event Handling

#### `sendPromptAsync()` / `sendPhotoPromptAsync()`
- Triggers OpenCode processing (non-blocking)
- Manages typing indicators
- Only handles errors from the trigger request itself

#### `handleMessageUpdated()`
- Receives completed response via SSE
- Formats and sends to Telegram
- Handles HTML formatting errors with plain text fallback
- Cleans up session state

## Configuration

### Environment Variables

```bash
# Required
TELEGRAM_BOT_TOKEN="your-bot-token"
TELEGRAM_CHAT_ID="your-chat-id"

# Optional
OPENCODE_BASE_URL="http://localhost:54321"  # default
OPENCODE_DIRECTORY="/Users/master"           # default: "."
TELEGRAM_DEBOUNCE_MS="1000"                  # default: 1000ms
```

### Startup

```bash
# Using start script
./start-bot.sh

# Or manually
source ~/.opencode-telegram-credentials
./opencode-telegram
```

## Session Management

### Directory Consistency

**Important**: Bot and TUI must use the same `OPENCODE_DIRECTORY` to see the same sessions.

- TUI uses directory specified at launch
- Bot uses `OPENCODE_DIRECTORY` environment variable
- Default: current directory (`.`)

### Session State

- `SessionIdle`: Ready for new messages
- `SessionBusy`: Processing a request
- `SessionError`: Last request failed

## Message Flow Details

### Text Messages

1. User sends message
2. `HandleUserMessage` → debounce buffer (1 second default)
3. `flushDebounceBuffer` → creates thinking message ("⏳ Processing...")
4. `sendPromptAsync` → triggers OpenCode (async)
5. SSE receives `message.updated` event
6. `handleMessageUpdated` → formats and sends to Telegram
7. Thinking message is replaced with actual response

### Photo Messages

Same flow as text, but:
- Downloads photo from Telegram
- Encodes as base64
- Sends as multipart message (image + optional caption)

## Error Handling

### HTML Formatting Errors

When Telegram rejects HTML formatting (e.g., unescaped special characters):

1. Attempt to send with HTML formatting
2. On error → retry with plain text
3. Log warning but ensure message is delivered

### Timeout Strategy

- **SendPrompt timeout**: 60 seconds (trigger only)
- **SSE connection**: No timeout (persistent connection)
- **Typing indicator**: Refreshed every 4 seconds while busy

## Debugging

### Logs Location

```bash
~/Library/Logs/opencode-telegram/stdout.log
~/Library/Logs/opencode-telegram/stderr.log
```

### Useful Log Patterns

```bash
# Check session handling
grep "BRIDGE.*session" stderr.log

# Check SSE events
grep "handleMessageUpdated" stderr.log

# Check errors
grep "ERROR\|WARN" stderr.log
```

## Architecture Decisions

### Q: Why not use long-polling instead of SSE?
A: SSE provides true push notifications with lower latency and better connection management.

### Q: Why 60-second timeout for SendPrompt?
A: We only need enough time to trigger the request. The actual processing happens asynchronously, and we receive results via SSE.

### Q: Why debounce user messages?
A: Prevents rapid-fire messages from creating multiple OpenCode sessions. Users often send multi-line messages as separate Telegram messages.

### Q: Why separate HTML and plain text send methods?
A: Telegram's HTML parser is strict. Having a plain text fallback ensures messages always get delivered, even if formatting fails.
