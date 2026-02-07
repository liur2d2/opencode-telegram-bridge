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
