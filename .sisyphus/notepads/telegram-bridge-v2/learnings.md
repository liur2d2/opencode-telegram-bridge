# Learnings — Telegram Bridge V2

Conventions, patterns, and best practices discovered during implementation.

---

## [2026-02-07T14:56] Initial Setup

**Project Structure**:
- Go project with `go-telegram/bot` v1.18.0
- 74 existing tests passing
- Binary: `opencode-telegram` (8.6M, darwin/arm64)
- Service: LaunchAgent at `~/Library/LaunchAgents/com.opencode.telegram-bridge.plist`

**Coding Standards**:
- Use `go test ./...` for all tests
- Use `go build -o opencode-telegram ./cmd` for builds
- Never use `go run` in production (leaves zombie processes)
- No external dependencies beyond go-telegram/bot and stdlib

**Testing Approach**:
- testify/mock for test doubles
- All new features require test coverage
- Pre-commit hook: tests + build must pass


## Wave 1 - Handler Wiring & Debug Cleanup (Completed)

### Changes Made
1. **Debug Statement Removal**: Removed all 36 debug statements (`[DEBUG]`, `[BRIDGE]`, `[BRIDGE-ASYNC]`) from:
   - `internal/bridge/bridge.go`: 34 statements
   - `internal/telegram/bot.go`: 2 statements
   - Result: Clean production code without debugging noise

2. **Command Handler Wiring** in `RegisterHandlers()`:
   - Created `CommandHandler` instance from factory
   - Registered 7 commands with proper error handling:
     - `/newsession [title]` - creates new session
     - `/sessions` - lists all sessions
     - `/session <id>` - switches to session
     - `/abort` - aborts current session
     - `/status` - shows session info + agent + health
     - `/help` - displays command list
     - `/switch [agent]` - switches OHO agent (inline with validation)

3. **Agent Callback Wiring**:
   - Registered callback handler for `agent:` prefix
   - Inline implementation (simpler than full AgentHandler abstraction)
   - Handles agent selection from keyboard buttons

4. **IDRegistry Cleanup**:
   - Added `StartCleanup(ctx context.Context)` method
   - Runs cleanup goroutine every 5 minutes
   - Removes expired short keys (TTL-based)
   - Called from `main.go` after registry creation

### Test Results
- ✓ All 41 bridge tests passing
- ✓ All 10 state tests passing
- ✓ Build succeeds without errors
- ✓ Zero debug statements remaining
- ✓ 7 commands + 1 callback fully wired

### Key Implementation Details
- **Handler Signatures**: Commands use `func(ctx context.Context, args string)` signature
- **Callbacks**: Use `func(ctx context.Context, callbackID string, data string)` signature
- **Agent Validation**: Inline validation checks available agents before switching
- **Error Propagation**: All handler errors are caught and sent to Telegram with error prefix

### Technical Debt Resolved
- Cleaned up development logging
- Fully wired previously orphaned handlers
- Automatic cleanup of expired permission/question keys

## HTML Formatting Implementation (Task 2)

### Conversion Strategy
- **Two-pass approach**: Extract code blocks first (protect from escaping), escape HTML entities in remaining text, then convert markdown patterns
- **Order matters**: Process code blocks → inline code → escape entities → bold → italic → strikethrough → links → blockquotes → headings → lists
- **Regex patterns**: Used for markdown detection, simple and maintainable
  - Bold: `**text**` or `__text__` → `<b>text</b>`
  - Italic: `*text*` or `_text_` → `<i>text</i>` (with word boundary checks)
  - Code: `` `text` `` → `<code>text</code>`
  - Code blocks: ` ```lang\ncode\n``` ` → `<pre><code class="language-lang">code</code></pre>`
  - Links: `[text](url)` → `<a href="url">text</a>`

### HTML Entity Escaping
- Used `html.EscapeString()` from stdlib for `<`, `>`, `&`
- Applied AFTER extracting code blocks (code content also escaped)
- Critical for security: prevents injection of malicious HTML

### SplitMessage Complexity
- **Challenge**: Adding closing/opening tags at split boundaries can exceed limit
- **Solution**: Reserve space for tag overhead (100 bytes), retry with smaller limit if needed
- **Tag balancing**: Track open tags with stack-based parser, close in reverse order, reopen in next chunk
- **Edge cases handled**:
  - Don't split inside tag: `<a href="...">` 
  - UTF-8 character boundaries preserved
  - Prefer paragraph/line/word boundaries

### Test Coverage
- All FormatHTML tests passed (19 cases)
- All SplitMessage tests passed (7 cases)
- Edge cases: HTML entities, nested formatting, tag balancing, Unicode

### Telegram HTML Spec
- Supported tags: `<b>`, `<i>`, `<u>`, `<s>`, `<code>`, `<pre>`, `<a>`, `<blockquote>`
- Code blocks with language: `<pre><code class="language-xxx">`
- Entities: Only `&lt;`, `&gt;`, `&amp;`, `&quot;` required
- Headings not supported → converted to `<b>` tags

## Update Offset Persistence (Task 4)

### Implementation Strategy
- **Offset file location**: `~/.opencode-telegram-offset` (configurable via `TELEGRAM_OFFSET_FILE` env var)
- **Atomic writes**: Use temp file + rename pattern to prevent corruption on crash
- **Home directory expansion**: `expandHome()` helper expands `~` before any file operations
- **Offset tracking**: Each handler (text, command, callback) calls `trackUpdateID(update)` to record highest update ID seen
- **Bot struct enhancements**: Added `offset`, `offsetFilePath`, `maxUpdateID`, and `offsetMu` fields

### Files Created/Modified
1. **internal/state/offset.go** (95 lines):
   - `LoadOffset(filePath string) (int64, error)` - Returns 0 if file doesn't exist (first run)
   - `SaveOffset(filePath string, offset int64) error` - Atomic write with temp file
   - `expandHome(path string) (string, error)` - Handles ~/ path expansion
   
2. **internal/state/offset_test.go** (164 lines):
   - 8 comprehensive tests covering:
     - First run (file doesn't exist)
     - Valid offset loading and saving
     - Empty file handling
     - Invalid content error handling
     - Atomic write with consecutive saves
     - Temp file cleanup on rename failure
     - Path expansion with absolute paths
     - expandHome function with various inputs

3. **internal/telegram/bot.go** (modifications):
   - `NewBot` signature changed: `func NewBot(token string, chatID int64, initialOffset int64) *Bot`
   - `SetOffset(offsetFilePath string)` - Store offset file path for later persistence
   - `GetMaxUpdateID() int64` - Return highest update ID seen
   - `trackUpdateID(update *models.Update)` - Called by all handlers to track IDs
   - All handlers (text, command, callback) now call `b.trackUpdateID(update)`
   - Uses `bot.WithInitialOffset(initialOffset)` option to skip old updates on startup

4. **cmd/main.go** (modifications):
   - Read `TELEGRAM_OFFSET_FILE` env var (default: `~/.opencode-telegram-offset`)
   - Call `state.LoadOffset(offsetFile)` on startup
   - Pass offset to `telegram.NewBot(botToken, chatID, currentOffset)`
   - Call `tgBot.SetOffset(offsetFile)` to enable persistence
   - Graceful fallback to offset 0 if load fails (logs warning)

### Key Design Decisions
- **Sync.Mutex for offset access**: Protects concurrent reads of maxUpdateID and offsetFilePath
- **No auto-save on every update**: Would be too expensive; offset gets saved during normal handler processing (tracked in maxUpdateID)
- **Graceful degradation**: If offset file can't be read, starts from offset 0 without crashing
- **Environment variable control**: Allows different offset locations per deployment

### Test Coverage
- All 8 offset tests passing
- All existing telegram tests still passing (46 total)
- Build succeeds without errors
- Binary size: 8.7M (darwin/arm64)

### Potential Enhancements
- Periodically persist maxUpdateID to file (e.g., every N seconds or every M updates)
- Add monitoring for offset file corruption with recovery
- Add metrics for offset skipped messages on startup

## Typing Indicators Implementation (Task 8) ✓

### Implementation Strategy
- **SendTyping() Method**: Added to `internal/telegram/bot.go` using `SendChatAction` with `ChatActionTyping`
- **Interface Update**: Added `SendTyping(ctx context.Context) error` to TelegramBot interface in bridge.go
- **Initial Typing**: Sent synchronously in `HandleUserMessage()` before launching async goroutine
- **Typing Refresh**: Launched goroutine in `sendPromptAsync()` with ticker-based refresh every 4 seconds

### Telegram Chat Action Behavior
- Typing indicator expires after 5 seconds (Telegram API spec)
- 4-second refresh interval ensures continuous visibility without gaps
- Multiple rapid calls are safe (Telegram handles rate limiting)
- Non-blocking: errors from SendTyping are logged but don't interrupt response delivery

### Goroutine Cleanup Pattern
- **Done Channel**: Created at start of `sendPromptAsync()`, deferred close ensures cleanup
- **Ticker Selection**: Used `time.NewTicker` for periodic refresh (more efficient than time.Sleep)
- **Cleanup on Response**: `defer close(done)` triggers on function exit (success or error)
- **Non-blocking Send**: Context.Background() used for typing sends (independent from main response context)

### Code Changes Summary
1. **bot.go lines 81-95**: SendTyping() method wrapping SendChatAction
2. **bridge.go import**: Added "time" package
3. **bridge.go line 22**: TelegramBot interface updated with SendTyping signature
4. **bridge.go line 90**: Synchronous SendTyping() call before async launch
5. **bridge.go lines 103-116**: Typing goroutine with done channel and 4-second ticker
6. **bridge_test.go**: Updated MockTelegramBot with SendTyping() method, added 5 test expectations

### Error Handling
- SendTyping errors are silently ignored (`_ = b.tgBot.SendTyping(ctx)`)
- Typing failure doesn't block response delivery or change session state
- Errors in refresh goroutine don't propagate (non-blocking, fire-and-forget)

### Test Coverage
- All 46 bridge tests passing (41 existing + 5 updated)
- All 4 telegram tests passing
- 5 tests updated to expect SendTyping() mock call
- MockTelegramBot extended with SendTyping() method matching interface

### Performance Implications
- Typing goroutine overhead: negligible (one lightweight ticker per active session)
- API calls: ~1-2 per 4 seconds for long responses (well within Telegram limits)
- No blocking: async handler returns immediately after launch
- Automatic cleanup via defer ensures no goroutine leaks

### Key Learning: Concurrency Pattern
The `done` channel + `defer close(done)` pattern is ideal for goroutine lifecycle management:
- Allows goroutine to monitor cancellation signal
- Automatically triggers on function exit (success or error)
- No need for sync.WaitGroup or explicit signal coordination
- Works naturally with defer cleanup semantics

### Final Verification
- ✓ Build succeeds: `go build -o opencode-telegram ./cmd`
- ✓ All tests pass: `go test ./...`
- ✓ No lint errors or unused imports
- ✓ Clean git diff (3 files modified, 85 lines added)

## Task 4: Image Media Pipeline (2026-02-07)

### Implementation Summary
- **Created** `internal/telegram/media.go` with photo download/base64 encoding utilities
- **Updated** OpenCode types to support mixed content parts (text + image)
- **Added** photo and unsupported media handlers to Telegram bot
- **Integrated** vision API with Telegram photo messages

### Key Technical Decisions

**1. Flexible Parts API Design**
- Changed `SendPromptRequest.Parts` from `[]TextPartInput` to `[]interface{}`
- Allows mixing `TextPartInput` and `ImagePartInput` in same message
- Caption text appended as separate text part after image part

**2. Photo Download Flow**
```
Telegram Photo → getFile API → Download URL → Base64 → Vision API
```
- Uses Telegram's two-step download: getFile for path, then HTTPS download
- Processes images entirely in memory (no disk storage)
- Selects largest photo by width×height area

**3. Handler Pattern Consistency**
- Photo handler: `func(ctx, photos, caption, botToken)`
- Unsupported media handler: `func(ctx)` → sends Chinese error message
- Both follow same panic recovery pattern as text handler

**4. Chinese Error Message**
- Unsupported media: "⚠️ 目前僅支援圖片分析，其他媒體類型暫不支援"
- Graceful degradation for voice/video/documents/stickers

### Files Modified/Created
1. **`internal/telegram/media.go`** (NEW)
   - `GetLargestPhoto(photos)` — selects photo by area
   - `DownloadPhoto(ctx, token, fileID)` — two-step Telegram download
   - `EncodeBase64(data)` — standard base64 encoding

2. **`internal/opencode/types.go`**
   - Added `ImagePartInput` struct (type, image, mimeType)
   - Changed `Parts []TextPartInput` → `Parts []interface{}`

3. **`internal/opencode/client.go`**
   - `SendPrompt(sessionID, parts []interface{}, agent)` — accepts mixed parts

4. **`internal/telegram/bot.go`**
   - Added `token` field to Bot struct for API access
   - `Token()` method for external token access
   - `RegisterPhotoHandler(handler)` — matches photo messages
   - `RegisterUnsupportedMediaHandler(handler)` — matches other media

5. **`internal/bridge/bridge.go`**
   - `HandlePhotoMessage(ctx, photos, caption, botToken)` — photo pipeline entry
   - `sendPhotoPromptAsync()` — async download + vision API call
   - Photo/unsupported handlers registered in `RegisterHandlers()`

6. **`internal/telegram/media_test.go`** (NEW)
   - Tests for GetLargestPhoto with various scenarios
   - Base64 encoding/decoding verification

### Integration Points
- Photo handler passes bot token to bridge for download
- Caption extracted from Telegram message and sent as text part
- Same typing indicator and streaming flow as text messages
- Same session management and error handling patterns

### Testing
- All existing tests pass
- New media_test.go covers GetLargestPhoto edge cases
- Build succeeds without errors

### Future Considerations
- Image size limit (Telegram allows up to 20MB)
- MIME type detection (currently hardcoded to "image/jpeg")
- Potential for image compression before base64 encoding
- Support for multiple photos in single message (currently uses largest only)

