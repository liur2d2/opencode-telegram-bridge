# Telegram Bridge V2 — Feature Parity Upgrade

## TL;DR

> **Quick Summary**: Upgrade opencode-telegram-bridge from a basic text relay to a feature-rich Telegram bot matching key OpenClaw capabilities. Includes Markdown→HTML rendering, streaming replies, media handling (image vision), typing indicators, message dedup/debounce, inline keyboards, reaction tracking, sticker support, webhook mode, proxy support, multi-account, multi-agent routing, and all existing bug fixes.
>
> **Deliverables**:
> - Markdown→HTML text rendering with ParseMode "HTML"
> - Streaming AI replies via SSE `message.part.updated` progressive editing
> - Image media pipeline (download → base64 → OpenCode vision API)
> - Typing indicators during AI processing
> - Update offset persistence (survive restarts)
> - Message fragment reassembly (long text auto-merge)
> - Inbound debounce (merge rapid consecutive messages)
> - Inline keyboard model selection (paginated)
> - Reaction tracking (emoji → agent notification)
> - DM pairing security (pairing codes + approval)
> - Webhook mode (alternative to polling)
> - SOCKS/HTTP proxy support
> - Sticker search & cache
> - Multi-account bot support
> - Multi-agent routing (per-chat agent assignment)
> - Forum topic session isolation
> - Wired commands, agent switching, question handler
> - Debug log cleanup + system prompt removal
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: Task 1 (fix existing) → Task 2 (HTML) → Task 4 (streaming) → Task 5 (media)

---

## Context

### Original Request

User wants ALL feature gaps between our Go-based opencode-telegram-bridge and OpenClaw's mature Telegram integration closed. User said: "功能差距向下所述我們沒有的功能我都想要" (I want all the missing features listed).

### Interview Summary

**Key Decisions**:
- **Media**: Support image analysis (Telegram photo → download → OpenCode vision API). Voice/video/documents: graceful "unsupported" reply for now.
- **Groups**: NOT needed. Single-user DM only. Skip group policy, @mention gating, forum topic isolation.
- **System prompt**: Remove hardcoded "回覆使用繁體中文" from client.go. Let OpenCode's own config handle it.
- **Streaming**: SSE-only via `message.part.updated` events (already have SSE infrastructure).
- **Server password**: Remove unused config field.

**Research Findings**:
- OpenClaw uses grammY (TS) with `ParseMode: "HTML"` — converting AI markdown to HTML is far simpler than MarkdownV2 escaping
- OpenClaw's message pipeline: sequentialize → throttle → dedup → fragment reassembly → media buffer → debounce → dispatch → stream reply
- go-telegram/bot library supports all needed Telegram Bot API features natively
- OpenCode SSE already defines `message.part.updated` event type with `delta` field for streaming

### Metis Review

**Identified Gaps (addressed)**:
- Media scope clarified: images only (vision), not full media suite
- Group scope clarified: DM only, no group features needed
- System prompt: remove hardcoded, use env var or OpenCode config
- Server password: remove dead code
- Streaming: SSE-only approach confirmed

**User Decisions (post-Metis)**:
- **Groups**: Explicitly declined → skip group handling, forum topics, @mention
- **Media**: Support image analysis → full image pipeline needed
- **System prompt**: Remove hardcoded → let OpenCode config handle
- **Since groups not needed**: Skip DM pairing (single user), multi-agent routing per chat, forum topic isolation
- **Still wanted**: Multi-account (multiple bots), inline model selection, reactions, stickers, webhook, proxy, debounce, fragment reassembly

---

## Work Objectives

### Core Objective
Transform the bridge from a plain-text relay into a production-quality Telegram bot with rich formatting, streaming responses, image support, and advanced Telegram features.

### Concrete Deliverables
- `internal/telegram/format.go`: Markdown→HTML converter
- `internal/telegram/bot.go`: HTML ParseMode, typing indicators, reactions, sticker support, webhook mode, proxy
- `internal/bridge/bridge.go`: Streaming handler, fragment reassembly, debounce, update offset persistence
- `internal/bridge/commands.go`: Wired command handlers
- `internal/bridge/agent.go`: Wired agent switching
- `internal/bridge/question.go`: Complete question handler with keyboard UI
- `internal/bridge/model.go`: Model selection with paginated inline keyboard
- `internal/opencode/client.go`: System prompt removal, image parts support
- `internal/telegram/media.go`: Image download + processing
- `internal/state/state.go`: Update offset, debounce state, multi-account
- Config/env updates for new features

### Definition of Done
- [ ] All 74 existing tests still pass: `go test ./...`
- [ ] New features have corresponding test coverage
- [ ] Binary builds without errors: `go build -o opencode-telegram ./cmd`
- [ ] LaunchAgent restart works: unload + load plist
- [ ] Telegram bot responds with HTML-formatted messages
- [ ] Streaming replies show progressive updates
- [ ] Image sent to bot triggers vision analysis response

### Must Have
- Markdown→HTML conversion with ParseMode "HTML"
- Streaming replies via SSE message.part.updated
- Image media pipeline (photo → OpenCode vision)
- Typing indicators
- Update offset persistence
- All existing commands wired and working
- Question handler with inline keyboards
- Agent switching wired
- Debug log cleanup

### Must NOT Have (Guardrails)
- ❌ Group message handling (user explicitly declined)
- ❌ Forum topic isolation (not needed without groups)
- ❌ @mention gating (not needed without groups)
- ❌ Per-group config/allowlists (not needed)
- ❌ Full media suite (voice transcription, video processing) — only images for now
- ❌ External dependencies beyond go-telegram/bot and stdlib
- ❌ Database persistence (in-memory + file offset only)
- ❌ Multi-user support beyond multi-account bots (each bot = single chat ID)
- ❌ Hardcoded system prompts in client.go

---

## Verification Strategy

> **UNIVERSAL RULE: ZERO HUMAN INTERVENTION**
>
> ALL tasks in this plan MUST be verifiable WITHOUT any human action.

### Test Decision
- **Infrastructure exists**: YES (go test, 74 existing tests)
- **Automated tests**: YES (tests-after — add tests for each new feature)
- **Framework**: `go test` with testify/mock

### Agent-Executed QA Scenarios (MANDATORY — ALL tasks)

**Verification Tool by Deliverable Type:**

| Type | Tool | How Agent Verifies |
|------|------|-------------------|
| **Go code** | Bash (`go test`, `go build`, `go vet`) | Run tests, build, vet |
| **Telegram bot** | Bash (curl Telegram API) + log monitoring | Send test message via API, monitor stdout.log |
| **Integration** | Bash (launchctl + tail logs) | Restart service, send message, verify response in logs |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — Fix Existing Bugs):
├── Task 1: Wire commands + agent switching + cleanup debug logs
├── Task 2: Markdown→HTML converter + ParseMode switch
└── Task 3: Remove hardcoded system prompt + dead server password code

Wave 2 (After Wave 1):
├── Task 4: Streaming replies via SSE message.part.updated
├── Task 5: Image media pipeline (download → vision API)
├── Task 6: Typing indicators
└── Task 7: Update offset persistence

Wave 3 (After Wave 2):
├── Task 8: Complete question handler with inline keyboards
├── Task 9: Message fragment reassembly
├── Task 10: Inbound debounce
└── Task 11: IDRegistry cleanup goroutine

Wave 4 (After Wave 3):
├── Task 12: Inline keyboard model selection (paginated)
├── Task 13: Reaction tracking
└── Task 14: Sticker search & send

Wave 5 (After Wave 4):
├── Task 15: Webhook mode (alternative to polling)
├── Task 16: Proxy support (SOCKS/HTTP)
└── Task 17: Multi-account bot support

Wave 6 (Final):
├── Task 18: Multi-agent routing
└── Task 19: Final integration test + rebuild + restart
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 | None | 8, 12, 18 | 2, 3 |
| 2 | None | 4, 9 | 1, 3 |
| 3 | None | 5 | 1, 2 |
| 4 | 2 | 19 | 5, 6, 7 |
| 5 | 3 | 19 | 4, 6, 7 |
| 6 | None (Wave 2 grouping) | 19 | 4, 5, 7 |
| 7 | None (Wave 2 grouping) | 19 | 4, 5, 6 |
| 8 | 1 | 19 | 9, 10, 11 |
| 9 | 2 | 19 | 8, 10, 11 |
| 10 | None (Wave 3 grouping) | 19 | 8, 9, 11 |
| 11 | None (Wave 3 grouping) | 19 | 8, 9, 10 |
| 12 | 1 | 19 | 13, 14 |
| 13 | None (Wave 4 grouping) | 19 | 12, 14 |
| 14 | None (Wave 4 grouping) | 19 | 12, 13 |
| 15 | None (Wave 5 grouping) | 17 | 16 |
| 16 | None (Wave 5 grouping) | 17 | 15 |
| 17 | 15, 16 | 18 | None |
| 18 | 1, 17 | 19 | None |
| 19 | ALL | None | None (final) |

### Agent Dispatch Summary

| Wave | Tasks | Recommended Agents |
|------|-------|-------------------|
| 1 | 1, 2, 3 | 3 parallel sisyphus-junior (quick category) |
| 2 | 4, 5, 6, 7 | 4 parallel sisyphus-junior (unspecified-high) |
| 3 | 8, 9, 10, 11 | 4 parallel sisyphus-junior (quick) |
| 4 | 12, 13, 14 | 3 parallel sisyphus-junior (quick) |
| 5 | 15, 16, 17 | 15+16 parallel, then 17 sequential |
| 6 | 18, 19 | sequential sisyphus |

---

## TODOs

- [x] 1. Wire Command Handlers + Agent Switching + Debug Log Cleanup

  **What to do**:
  - In `cmd/main.go`, call all command registration methods that are defined but never invoked:
    - `bridgeInstance.RegisterHandlers()` is called, but inside it only registers the text handler and permission callback
    - Add calls to register: `/newsession`, `/sessions`, `/session`, `/abort`, `/status`, `/help`, `/switch`
  - In `internal/bridge/bridge.go` `RegisterHandlers()`:
    - Wire `CommandHandler.HandleNewSession` to `/newsession` via `b.tgBot.RegisterCommandHandler("newsession", ...)`
    - Wire `CommandHandler.HandleListSessions` to `/sessions`
    - Wire `CommandHandler.HandleSwitchSession` to `/session`
    - Wire `CommandHandler.HandleAbort` to `/abort`
    - Wire `CommandHandler.HandleStatus` to `/status`
    - Wire `CommandHandler.HandleHelp` to `/help`
    - Wire `AgentHandler.HandleSwitch` to `/switch`
    - Wire `AgentHandler.HandleAgentCallback` to callback prefix `"agent:"`
  - Create `CommandHandler` and `AgentHandler` instances in `RegisterHandlers()` (they need references to ocClient, tgBot, state)
  - Remove all 41 `fmt.Printf("[DEBUG]..."` and `fmt.Printf("[BRIDGE]..."` debug log statements from:
    - `internal/bridge/bridge.go`
    - `internal/telegram/bot.go`
  - Keep `[BRIDGE-ASYNC]` error logs (convert to `log.Printf` for proper logging)
  - Start IDRegistry cleanup goroutine in `cmd/main.go`

  **Must NOT do**:
  - Do not change the handler function signatures
  - Do not add new dependencies
  - Do not modify test mocks (update tests to match)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`git-master`]
    - `git-master`: For atomic commit after wiring

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 8, 12, 18
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/bridge/bridge.go:349-387` — RegisterHandlers() current implementation (only text + permission callback)
  - `internal/bridge/commands.go:1-167` — All command handler methods (HandleNewSession, HandleListSessions, etc.)
  - `internal/bridge/agent.go:1-233` — AgentHandler with HandleSwitch, HandleAgentCallback
  - `internal/telegram/bot.go:135-149` — RegisterCommandHandler() method

  **API/Type References**:
  - `internal/telegram/bot.go:108-110` — TextHandler, CommandHandler, CallbackHandler type definitions
  - `internal/bridge/bridge.go:16-30` — TelegramBot and OpenCodeClient interfaces

  **Acceptance Criteria**:
  - [ ] `/newsession test` creates a new session and responds with session ID
  - [ ] `/sessions` lists all sessions
  - [ ] `/session <id>` switches to specified session
  - [ ] `/abort` aborts current session
  - [ ] `/status` shows session info, agent, health
  - [ ] `/help` shows command list
  - [ ] `/switch` shows agent selection keyboard
  - [ ] Clicking agent keyboard button switches agent
  - [ ] Zero `[DEBUG]` or `fmt.Printf("[BRIDGE]` lines remain in source
  - [ ] `go test ./...` passes
  - [ ] `go build -o opencode-telegram ./cmd` succeeds

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Commands are wired and respond
    Tool: Bash (go test + grep)
    Preconditions: Source code modified
    Steps:
      1. Run: grep -rn '\[DEBUG\]' internal/ cmd/ | grep -v _test.go
      2. Assert: zero matches (all debug logs removed)
      3. Run: grep -n 'RegisterCommandHandler' internal/bridge/bridge.go
      4. Assert: at least 7 registrations (newsession, sessions, session, abort, status, help, switch)
      5. Run: go test ./... -count=1
      6. Assert: all tests pass
      7. Run: go build -o opencode-telegram ./cmd
      8. Assert: build succeeds
    Expected Result: All commands wired, no debug logs, tests pass
    Evidence: Terminal output captured

  Scenario: IDRegistry cleanup runs
    Tool: Bash (grep)
    Steps:
      1. grep -n 'cleanup\|Cleanup\|ticker' cmd/main.go internal/state/registry.go
      2. Assert: cleanup goroutine started in main.go or registry.go
    Expected Result: TTL cleanup is active
    Evidence: Source code grep output
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire all command handlers, agent switching, and remove debug logs`
  - Files: `internal/bridge/bridge.go`, `internal/bridge/commands.go`, `internal/bridge/agent.go`, `internal/telegram/bot.go`, `cmd/main.go`, `internal/state/registry.go`
  - Pre-commit: `go test ./... && go build -o opencode-telegram ./cmd`

---

- [x] 2. Markdown→HTML Converter + ParseMode Switch

  **What to do**:
  - Rewrite `internal/telegram/format.go`:
    - Replace `FormatMarkdownV2()` with `FormatHTML()` function
    - Convert AI markdown to Telegram-compatible HTML:
      - `**bold**` / `__bold__` → `<b>bold</b>`
      - `*italic*` / `_italic_` → `<i>italic</i>`
      - `` `inline code` `` → `<code>inline code</code>`
      - ` ```lang\ncode\n``` ` → `<pre><code class="language-lang">code</code></pre>`
      - `~~strikethrough~~` → `<s>strikethrough</s>`
      - `[text](url)` → `<a href="url">text</a>`
      - `> blockquote` → `<blockquote>blockquote</blockquote>`
      - `# Heading` → `<b>Heading</b>\n` (Telegram has no heading tag)
      - `- list item` → `• list item`
      - `1. item` → `1. item` (keep as-is)
      - Escape HTML entities: `<`, `>`, `&` in non-tag content
    - Keep `SplitMessage()` working with HTML (must not split inside tags)
    - Update `SplitMessage()` to handle HTML tag balancing on splits (close open tags at split, reopen in next chunk)
  - Update `internal/telegram/bot.go`:
    - Change all three methods (SendMessage, SendMessageWithKeyboard, EditMessage) to use `ParseMode: models.ParseModeHTML`
  - Remove old `FormatMarkdownV2()` function and `markdownV2SpecialChars` variable

  **Must NOT do**:
  - Do not use external markdown parsing libraries (keep it simple, regex-based)
  - Do not handle nested formatting (e.g., bold inside italic) — single-level only is fine
  - Do not break SplitMessage's existing paragraph/line/word split logic

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
    - No special skills needed, pure Go string manipulation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4, 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/format.go:1-150` — Current FormatMarkdownV2 and SplitMessage implementations
  - `internal/telegram/format_test.go` — Existing test patterns for formatting

  **API/Type References**:
  - `internal/telegram/bot.go:39-79` — SendMessage, SendMessageWithKeyboard, EditMessage (currently plain text)
  - Telegram Bot API HTML format: `<b>`, `<i>`, `<code>`, `<pre>`, `<a href="">`, `<s>`, `<blockquote>`

  **External References**:
  - Telegram HTML spec: https://core.telegram.org/bots/api#html-style

  **Acceptance Criteria**:
  - [ ] `FormatHTML("**bold**")` returns `<b>bold</b>`
  - [ ] `FormatHTML("` `` `code` `` `")` returns `<code>code</code>`
  - [ ] Code blocks with language preserved: ` ```go\nfmt.Println()\n``` ` → `<pre><code class="language-go">...</code></pre>`
  - [ ] HTML entities escaped in plain text: `<script>` → `&lt;script&gt;`
  - [ ] `SplitMessage()` with HTML doesn't break tags mid-tag
  - [ ] All three bot methods use `ParseMode: models.ParseModeHTML`
  - [ ] `go test ./internal/telegram/...` passes with new tests
  - [ ] Old FormatMarkdownV2 function removed

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: HTML formatting converts correctly
    Tool: Bash (go test)
    Preconditions: format.go rewritten
    Steps:
      1. Run: go test ./internal/telegram/... -v -run TestFormatHTML
      2. Assert: all format conversion tests pass
      3. Run: go test ./internal/telegram/... -v -run TestSplitMessage
      4. Assert: split respects HTML tag boundaries
    Expected Result: All formatting and splitting tests pass
    Evidence: Test output captured

  Scenario: Bot uses HTML ParseMode
    Tool: Bash (grep)
    Steps:
      1. grep -n 'ParseMode' internal/telegram/bot.go
      2. Assert: all occurrences use models.ParseModeHTML or "HTML"
      3. grep -n 'MarkdownV2\|FormatMarkdownV2' internal/
      4. Assert: zero matches (old code removed)
    Expected Result: HTML mode active, no MarkdownV2 remnants
    Evidence: grep output
  ```

  **Commit**: YES
  - Message: `feat(telegram): replace MarkdownV2 with HTML rendering for AI responses`
  - Files: `internal/telegram/format.go`, `internal/telegram/format_test.go`, `internal/telegram/bot.go`
  - Pre-commit: `go test ./internal/telegram/...`

---

- [x] 3. Remove Hardcoded System Prompt + Dead Server Password Code

  **What to do**:
  - In `internal/opencode/client.go` `SendPrompt()`:
    - Remove the hardcoded `systemPrompt := "回覆使用繁體中文"` and `System: &systemPrompt`
    - The `SendPromptRequest.System` field should be `nil` (let OpenCode's config handle it)
  - In `internal/opencode/types.go`:
    - Keep the `System *string` field in `SendPromptRequest` (might be useful later)
  - In `cmd/main.go`:
    - Remove `OPENCODE_SERVER_PASSWORD` env var reading
  - In `internal/opencode/types.go` `Config`:
    - Remove `ServerPassword string` field
  - In `internal/opencode/client.go`:
    - Remove any references to `ServerPassword`
  - Update tests that reference ServerPassword

  **Must NOT do**:
  - Do not remove the `System` field from `SendPromptRequest` type (keep for future use)
  - Do not touch OpenCode's own config files (`~/.config/opencode/opencode.json`)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: [`git-master`]
    - `git-master`: Small, clean commit

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Task 5
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/opencode/client.go:181-190` — SendPrompt with hardcoded systemPrompt
  - `cmd/main.go:40-45` — env var reading for OPENCODE_SERVER_PASSWORD

  **API/Type References**:
  - `internal/opencode/types.go:6-10` — Config struct with ServerPassword
  - `internal/opencode/types.go:103-107` — SendPromptRequest with System field

  **Acceptance Criteria**:
  - [ ] No `"回覆使用繁體中文"` string in any Go source file
  - [ ] No `ServerPassword` in Config struct or main.go
  - [ ] `SendPromptRequest.System` field still exists in types.go
  - [ ] `go test ./...` passes
  - [ ] `go build -o opencode-telegram ./cmd` succeeds

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Hardcoded prompts and dead code removed
    Tool: Bash (grep + go test)
    Steps:
      1. grep -rn '回覆使用繁體中文' internal/ cmd/
      2. Assert: zero matches
      3. grep -rn 'ServerPassword' internal/ cmd/
      4. Assert: zero matches
      5. grep -n 'System.*string' internal/opencode/types.go
      6. Assert: System field still exists in SendPromptRequest
      7. go test ./... -count=1
      8. Assert: all tests pass
    Expected Result: Dead code removed, System field preserved
    Evidence: grep + test output
  ```

  **Commit**: YES
  - Message: `refactor(opencode): remove hardcoded system prompt and unused server password config`
  - Files: `internal/opencode/client.go`, `internal/opencode/types.go`, `cmd/main.go`
  - Pre-commit: `go test ./...`

---

- [ ] 4. Streaming Replies via SSE message.part.updated

  **What to do**:
  - In `internal/bridge/bridge.go`:
    - Add handler for `"message.part.updated"` SSE event in `HandleSSEEvent()` dispatch
    - Create `handleMessagePartUpdated(event)`:
      - Extract `delta` string from event properties (incremental text)
      - Extract `sessionID` from event properties
      - Maintain a text accumulator per session: `streamBuffers sync.Map` mapping sessionID → `*StreamBuffer`
      - `StreamBuffer` struct: `{ text string, lastEdit time.Time, thinkingMsgID int }`
      - On each delta: append to buffer, if >500ms since last edit OR buffer ends with `\n\n`, edit thinking message with `FormatHTML(buffer.text)`
      - Throttle edits to max 1 per 500ms (Telegram rate limits: ~30 edits/min per chat)
    - Modify `sendPromptAsync()`:
      - After launching SendPrompt, the SSE events will handle progressive updates
      - SendPrompt response is still the final version — use it to send the FINAL edit (ensures completeness)
      - Clear streamBuffer[sessionID] after final response
    - Add `streamBuffers sync.Map` to Bridge struct
  - In `internal/bridge/bridge.go` `handleMessageUpdated()`:
    - This is the FINAL message — always overwrite with complete response
    - Clear any stream buffer for this session
  - Handle edge case: if streaming edits fail (Telegram rejects edit of identical content), silently ignore `"message is not modified"` errors

  **Must NOT do**:
  - Do not edit more than once per 500ms (will hit Telegram rate limits)
  - Do not use polling — SSE only
  - Do not remove the existing handleMessageUpdated (it serves as the final/fallback)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
    - Pure Go concurrency with sync.Map and timers

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: Task 19
  - **Blocked By**: Task 2 (needs FormatHTML)

  **References**:

  **Pattern References**:
  - `internal/bridge/bridge.go:170-187` — HandleSSEEvent dispatch (add new case)
  - `internal/bridge/bridge.go:229-331` — handleMessageUpdated (final response handler)
  - `internal/opencode/sse.go:115-125` — Event type parsing including "message.part.updated"

  **API/Type References**:
  - `internal/opencode/types.go:183-190` — EventMessagePartUpdated with Delta *string field
  - `internal/bridge/bridge.go:38-48` — Bridge struct (add streamBuffers field)

  **Acceptance Criteria**:
  - [ ] `handleMessagePartUpdated` method exists and handles delta events
  - [ ] StreamBuffer struct accumulates text with 500ms throttle
  - [ ] Thinking message progressively updates with HTML-formatted partial response
  - [ ] Final `message.updated` event overwrites with complete response
  - [ ] "message is not modified" errors silently ignored
  - [ ] `go test ./internal/bridge/...` passes with streaming tests

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Streaming handler registered and compiles
    Tool: Bash (go build + grep)
    Steps:
      1. grep -n 'message.part.updated' internal/bridge/bridge.go
      2. Assert: case exists in HandleSSEEvent dispatch
      3. grep -n 'StreamBuffer\|streamBuffers' internal/bridge/bridge.go
      4. Assert: struct and sync.Map field exist
      5. go build -o opencode-telegram ./cmd
      6. Assert: builds successfully
      7. go test ./internal/bridge/... -v -run TestStreaming
      8. Assert: streaming tests pass
    Expected Result: Streaming handler exists and works
    Evidence: grep + build + test output
  ```

  **Commit**: YES
  - Message: `feat(bridge): add streaming replies via SSE message.part.updated with throttled edits`
  - Files: `internal/bridge/bridge.go`, `internal/bridge/bridge_test.go`
  - Pre-commit: `go test ./internal/bridge/...`

---

- [ ] 5. Image Media Pipeline (Download → OpenCode Vision API)

  **What to do**:
  - Create `internal/telegram/media.go`:
    - `DownloadPhoto(ctx, bot, fileID string) ([]byte, error)` — download photo via Telegram Bot API `getFile` + HTTPS download
    - `EncodeBase64(data []byte) string` — base64 encode for API
    - `GetLargestPhoto(photos []models.PhotoSize) models.PhotoSize` — pick highest resolution
  - Update `internal/opencode/types.go`:
    - Add `ImagePartInput` struct: `{ Type: "image", Image: string (base64), MimeType: string }`
    - Update `SendPromptRequest.Parts` to accept `[]interface{}` (both TextPartInput and ImagePartInput)
  - Update `internal/opencode/client.go` `SendPrompt()`:
    - Add optional `images []ImagePartInput` parameter or accept `parts []interface{}` directly
    - Include image parts in the request body
  - Update `internal/bridge/bridge.go`:
    - Add photo message handler in `RegisterHandlers()`:
      - Detect `update.Message.Photo` array
      - Download largest photo
      - Base64 encode
      - Send to OpenCode with both text caption (if any) and image part
    - For unsupported media (voice, video, document, sticker):
      - Reply with "⚠️ 目前僅支援圖片分析，其他媒體類型暫不支援"
  - Update `internal/telegram/bot.go`:
    - Add `RegisterPhotoHandler(handler PhotoHandler)` method
    - `PhotoHandler func(ctx context.Context, photos []models.PhotoSize, caption string)`

  **Must NOT do**:
  - Do not implement voice transcription or video processing
  - Do not store downloaded images permanently (process in memory, discard)
  - Do not download images larger than 20MB (Telegram's limit)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []
    - HTTP file download + base64 encoding + API integration

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6, 7)
  - **Blocks**: Task 19
  - **Blocked By**: Task 3 (system prompt removal affects SendPrompt)

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:112-133` — RegisterTextHandler pattern (adapt for photos)
  - `internal/opencode/client.go:181-224` — SendPrompt implementation (extend for image parts)
  - `internal/bridge/bridge.go:59-101` — HandleUserMessage pattern (adapt for image messages)

  **API/Type References**:
  - `internal/opencode/types.go:96-107` — TextPartInput and SendPromptRequest (extend)
  - go-telegram/bot `models.PhotoSize`: FileID, Width, Height, FileSize
  - Telegram Bot API `getFile`: returns file_path for HTTPS download

  **External References**:
  - Telegram getFile API: https://core.telegram.org/bots/api#getfile
  - OpenCode vision API: accepts `type: "image"` parts with base64 data

  **Acceptance Criteria**:
  - [ ] `internal/telegram/media.go` exists with DownloadPhoto, EncodeBase64, GetLargestPhoto
  - [ ] Photo handler registered in bridge
  - [ ] Sending a photo to bot triggers vision analysis response
  - [ ] Caption text is included alongside image
  - [ ] Unsupported media types get graceful error message
  - [ ] `go test ./...` passes

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Media module compiles and photo handler registered
    Tool: Bash (go build + grep)
    Steps:
      1. ls internal/telegram/media.go
      2. Assert: file exists
      3. grep -n 'Photo\|photo' internal/bridge/bridge.go
      4. Assert: photo handler registration exists
      5. grep -n 'ImagePartInput' internal/opencode/types.go
      6. Assert: image type defined
      7. go build -o opencode-telegram ./cmd
      8. Assert: builds successfully
      9. go test ./... -count=1
      10. Assert: all tests pass
    Expected Result: Image pipeline compiles and integrates
    Evidence: build + test output
  ```

  **Commit**: YES
  - Message: `feat(media): add image download and vision analysis pipeline`
  - Files: `internal/telegram/media.go`, `internal/telegram/media_test.go`, `internal/opencode/types.go`, `internal/opencode/client.go`, `internal/bridge/bridge.go`, `internal/telegram/bot.go`
  - Pre-commit: `go test ./...`

---

- [ ] 6. Typing Indicators

  **What to do**:
  - In `internal/telegram/bot.go`:
    - Add `SendTyping(ctx context.Context) error` method
    - Uses `bot.SendChatAction(ctx, &bot.SendChatActionParams{ ChatID: b.chatID, Action: models.ChatActionTyping })`
  - In `internal/bridge/bridge.go`:
    - After receiving user message and before launching async prompt, call `b.tgBot.SendTyping(ctx)`
    - In `sendPromptAsync()`, start a typing indicator goroutine:
      - Loop every 4 seconds (Telegram typing expires after 5s)
      - Send typing action
      - Stop when response is ready (use done channel)
    - When streaming, continue typing between edits if there's a gap >4s

  **Must NOT do**:
  - Do not send typing more than once per 4 seconds
  - Do not block the main message handler with typing
  - Do not leave typing goroutine running after response delivered

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 7)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:36-50` — SendMessage pattern (adapt for SendChatAction)
  - `internal/bridge/bridge.go:97-100` — Goroutine launch pattern

  **API/Type References**:
  - go-telegram/bot: `bot.SendChatActionParams`, `models.ChatActionTyping`
  - Telegram Bot API: sendChatAction expires after 5 seconds

  **Acceptance Criteria**:
  - [ ] `SendTyping()` method exists on Bot
  - [ ] Typing indicator sent before AI processing starts
  - [ ] Typing refreshed every 4s during long responses
  - [ ] Typing goroutine stops when response delivered
  - [ ] `go test ./...` passes

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Typing indicator exists
    Tool: Bash (grep + go build)
    Steps:
      1. grep -n 'SendTyping\|ChatActionTyping' internal/telegram/bot.go
      2. Assert: method and action constant exist
      3. grep -n 'SendTyping\|typing' internal/bridge/bridge.go
      4. Assert: typing called in message handling flow
      5. go build -o opencode-telegram ./cmd
      6. Assert: builds successfully
    Expected Result: Typing indicators implemented
    Evidence: grep + build output
  ```

  **Commit**: YES (group with Task 7)
  - Message: `feat(telegram): add typing indicators during AI processing`
  - Files: `internal/telegram/bot.go`, `internal/bridge/bridge.go`
  - Pre-commit: `go test ./...`

---

- [ ] 7. Update Offset Persistence

  **What to do**:
  - Create `internal/state/offset.go`:
    - `LoadOffset(filepath string) (int64, error)` — read last update_id from file
    - `SaveOffset(filepath string, offset int64) error` — write update_id to file (atomic write: write to .tmp then rename)
    - File path default: `~/.opencode-telegram-offset`
  - In `internal/telegram/bot.go`:
    - Add `SetOffset(offset int64)` method to configure bot's initial offset
    - Modify `Start()` to accept initial offset (or set via bot options)
    - After processing each update batch, save the highest update_id + 1
  - In `cmd/main.go`:
    - Load offset on startup
    - Pass to bot
  - Add `TELEGRAM_OFFSET_FILE` env var (optional, defaults to `~/.opencode-telegram-offset`)

  **Must NOT do**:
  - Do not use database for offset storage (plain file is sufficient)
  - Do not save offset on every single update (batch saves, max once per second)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 6)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/state/state.go` — File-based state pattern
  - `cmd/main.go:27-49` — Env var reading pattern

  **API/Type References**:
  - go-telegram/bot: `bot.WithUpdatesOffset(offset)` option
  - Telegram Bot API: getUpdates offset parameter

  **Acceptance Criteria**:
  - [ ] `internal/state/offset.go` exists with Load/Save functions
  - [ ] Offset file created on first run
  - [ ] After restart, old updates are NOT replayed
  - [ ] Atomic file write (tmp + rename)
  - [ ] `go test ./internal/state/...` passes

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Offset persistence compiles and tests pass
    Tool: Bash (go test)
    Steps:
      1. ls internal/state/offset.go
      2. Assert: file exists
      3. go test ./internal/state/... -v -run TestOffset
      4. Assert: offset load/save tests pass
      5. go build -o opencode-telegram ./cmd
      6. Assert: builds successfully
    Expected Result: Offset persistence works
    Evidence: test + build output
  ```

  **Commit**: YES (group with Task 6)
  - Message: `feat(state): add update offset persistence to survive restarts`
  - Files: `internal/state/offset.go`, `internal/state/offset_test.go`, `internal/telegram/bot.go`, `cmd/main.go`
  - Pre-commit: `go test ./...`

---

- [ ] 8. Complete Question Handler with Inline Keyboards

  **What to do**:
  - In `internal/bridge/question.go`:
    - Implement `HandleQuestionAsked(event opencode.Event)`:
      - Unmarshal `EventQuestionAsked` from event properties
      - For each question in `Questions[]`:
        - Build inline keyboard from `Options[]` (label as button text, short ID as callback data)
        - If `Custom` is true (default), add "✏️ 自訂回答" button
        - If `Multiple` is true, add checkmark toggle behavior
        - Format question text: Header + Question text
      - Send message with keyboard
      - Store question state in registry: shortKey → QuestionState
    - Implement `HandleQuestionCallback(ctx, shortKey, data string)`:
      - Parse callback data to extract selected option label
      - Call `ocClient.ReplyQuestion(requestID, []QuestionAnswer{selectedLabels})`
      - Edit message to show selected answer
    - Handle custom input: When "✏️ 自訂回答" is clicked, set a state flag. Next text message from user becomes the custom answer.
  - In `internal/bridge/bridge.go`:
    - Un-ignore `"question.asked"` event (currently lines 178-179)
    - Wire to `questionHandler.HandleQuestionAsked(event)`
    - Register question callback handler with prefix `"q:"`
  - In `internal/telegram/keyboard.go`:
    - `BuildQuestionKeyboard()` already exists — verify it works with real data
    - Add multi-select toggle support if `Multiple` is true

  **Must NOT do**:
  - Do not support multi-question (multiple questions in single request) with separate keyboards — send all as one message
  - Do not handle binary attachments in custom answers

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 10, 11)
  - **Blocks**: Task 19
  - **Blocked By**: Task 1 (needs wired callback infrastructure)

  **References**:

  **Pattern References**:
  - `internal/bridge/bridge.go:349-433` — Permission callback pattern (follow same pattern for questions)
  - `internal/bridge/question.go:1-51` — Existing stub implementation
  - `internal/telegram/keyboard.go:41-103` — BuildQuestionKeyboard and BuildPermissionKeyboard

  **API/Type References**:
  - `internal/opencode/types.go:13-44` — QuestionRequest, QuestionInfo, QuestionOption, QuestionAnswer
  - `internal/opencode/client.go:258-280` — ReplyQuestion, RejectQuestion methods
  - `internal/state/registry.go` — IDRegistry for short key mapping

  **Acceptance Criteria**:
  - [ ] `question.asked` SSE event triggers keyboard display
  - [ ] Clicking option sends ReplyQuestion to OpenCode
  - [ ] Message edited to show selected answer
  - [ ] Custom input mode works (click custom → type → sends)
  - [ ] `go test ./internal/bridge/...` passes

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Question handler compiles and is wired
    Tool: Bash (grep + go build)
    Steps:
      1. grep -n 'question.asked' internal/bridge/bridge.go
      2. Assert: event handler dispatches to question handler (not ignored)
      3. grep -n 'HandleQuestionAsked\|HandleQuestionCallback' internal/bridge/question.go
      4. Assert: both methods have implementation (not empty)
      5. grep -n '"q:"' internal/bridge/bridge.go
      6. Assert: callback prefix registered
      7. go test ./... -count=1
      8. Assert: all tests pass
    Expected Result: Question handling fully implemented
    Evidence: grep + test output
  ```

  **Commit**: YES
  - Message: `feat(bridge): implement question handler with inline keyboard support`
  - Files: `internal/bridge/question.go`, `internal/bridge/question_test.go`, `internal/bridge/bridge.go`, `internal/telegram/keyboard.go`
  - Pre-commit: `go test ./...`

---

- [ ] 9. Message Fragment Reassembly

  **What to do**:
  - In `internal/bridge/bridge.go`:
    - Add `fragmentBuffers sync.Map` to Bridge struct: sessionID → `*FragmentBuffer`
    - `FragmentBuffer` struct: `{ parts []string, lastReceived time.Time, timer *time.Timer }`
    - When a message arrives:
      - If message length ≥ 3900 chars (near Telegram's 4096 limit, indicating possible split):
        - Buffer the message
        - Start/reset a 1.5s timer
        - On timer expiry: concatenate all buffered parts → send as single prompt
      - If message length < 3900 chars and buffer has content:
        - If within 1.5s of last fragment: add to buffer, reset timer
        - Otherwise: flush buffer first, then handle new message normally
      - If no buffer and message < 3900 chars: handle normally (no change)
    - Add `flushFragmentBuffer(sessionID string)` method

  **Must NOT do**:
  - Do not buffer ALL messages (only those ≥3900 chars or arriving within 1.5s of a large message)
  - Do not hold fragments indefinitely (1.5s max wait)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 10, 11)
  - **Blocks**: Task 19
  - **Blocked By**: Task 2 (uses FormatHTML in response)

  **References**:

  **Pattern References**:
  - `internal/bridge/bridge.go:59-101` — HandleUserMessage (insert fragment check before prompt)
  - OpenClaw fragment detection: buffers messages ≥4000 chars, merges within 1.5s / 1 message_id gap

  **Acceptance Criteria**:
  - [ ] Messages ≥3900 chars are buffered
  - [ ] Consecutive fragments within 1.5s are merged
  - [ ] Merged text sent as single prompt
  - [ ] Normal short messages unaffected
  - [ ] Buffer flushes after 1.5s timeout
  - [ ] `go test ./internal/bridge/...` passes

  **Commit**: YES
  - Message: `feat(bridge): add message fragment reassembly for long Telegram texts`
  - Files: `internal/bridge/bridge.go`, `internal/bridge/bridge_test.go`
  - Pre-commit: `go test ./internal/bridge/...`

---

- [ ] 10. Inbound Debounce

  **What to do**:
  - In `internal/bridge/bridge.go`:
    - Add `debounceBuffers sync.Map` to Bridge struct: sessionID → `*DebounceBuffer`
    - `DebounceBuffer` struct: `{ messages []string, timer *time.Timer, lastReceived time.Time }`
    - Configurable debounce window via env var `TELEGRAM_DEBOUNCE_MS` (default: 1000ms)
    - When a message arrives (after fragment check):
      - If a debounce buffer exists for this session and within debounce window:
        - Append message to buffer
        - Reset timer
      - Otherwise:
        - Create new buffer with this message
        - Start timer (debounce window)
      - On timer expiry: concatenate messages with `\n`, send as single prompt
    - Integrate with fragment reassembly: fragments resolve first, then debounce applies to resolved text

  **Must NOT do**:
  - Do not debounce if only one message (send immediately after timer)
  - Do not make debounce window too long (max 3000ms via env var)
  - Do not debounce command messages (only text messages)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9, 11)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/bridge/bridge.go:59-101` — HandleUserMessage (integrate after fragment check)
  - OpenClaw debounce: configurable per-conversation debounce window

  **Acceptance Criteria**:
  - [ ] Rapid consecutive messages merged into single prompt
  - [ ] Debounce window configurable via `TELEGRAM_DEBOUNCE_MS`
  - [ ] Single message sent immediately after debounce timer
  - [ ] Commands not debounced
  - [ ] `go test ./internal/bridge/...` passes

  **Commit**: YES
  - Message: `feat(bridge): add inbound message debouncing to merge rapid messages`
  - Files: `internal/bridge/bridge.go`, `internal/bridge/bridge_test.go`, `cmd/main.go`
  - Pre-commit: `go test ./internal/bridge/...`

---

- [ ] 11. IDRegistry Cleanup Goroutine

  **What to do**:
  - In `internal/state/registry.go`:
    - Add `StartCleanup(ctx context.Context)` method
    - Launch goroutine that runs every 5 minutes
    - Iterates `ttl` map, deletes expired entries from `mappings`, `reverse`, and `ttl`
    - Stops when context is cancelled
  - In `cmd/main.go`:
    - Call `registry.StartCleanup(ctx)` after creating registry

  **Must NOT do**:
  - Do not clean up more than once per 5 minutes
  - Do not block on cleanup

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9, 10)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/state/registry.go:50-88` — Existing Register/Resolve/cleanup methods (cleanup exists but never called)

  **Acceptance Criteria**:
  - [ ] `StartCleanup()` method exists
  - [ ] Expired entries are removed after TTL
  - [ ] Cleanup stops on context cancellation
  - [ ] `go test ./internal/state/...` passes

  **Commit**: YES (group with Task 9 or 10)
  - Message: `fix(state): start IDRegistry TTL cleanup goroutine`
  - Files: `internal/state/registry.go`, `cmd/main.go`
  - Pre-commit: `go test ./internal/state/...`

---

- [ ] 12. Inline Keyboard Model Selection (Paginated)

  **What to do**:
  - Create `internal/bridge/model.go`:
    - `ModelHandler` struct with ocClient, tgBot, state references
    - `HandleModelCommand(ctx, args string)`:
      - Fetch available models from OpenCode API (need to add `ListModels()` endpoint if available, or use config)
      - Build paginated inline keyboard (max 8 buttons per page)
      - Page navigation buttons: `◀️ Prev` / `▶️ Next`
      - Callback data format: `"mdl:page:N"` for navigation, `"mdl:sel:modelID"` for selection
    - `HandleModelCallback(ctx, callbackID, data string)`:
      - Parse callback data
      - If page navigation: edit message with new page
      - If model selection: store selected model in state, edit message to confirm
    - Model list source: read from oh-my-opencode.json config OR hardcode known models
  - In `internal/bridge/bridge.go`:
    - Register `/model` command
    - Register `"mdl:"` callback prefix
  - In `internal/state/state.go`:
    - Add `currentModel string` field
    - Add `GetCurrentModel()` / `SetCurrentModel()` methods
  - In `internal/opencode/client.go` `SendPrompt()`:
    - If model is set in state, include in request (check if OpenCode API supports per-message model override)

  **Must NOT do**:
  - Do not support more than 50 models (reasonable upper limit for pagination)
  - Do not modify oh-my-opencode.json from the bridge

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 13, 14)
  - **Blocks**: Task 19
  - **Blocked By**: Task 1 (needs wired command infrastructure)

  **References**:

  **Pattern References**:
  - `internal/bridge/agent.go:1-233` — Agent switching with keyboard (follow same pattern)
  - `internal/telegram/keyboard.go` — Keyboard building patterns
  - OpenClaw model selection: `mdl_prov` → `mdl_list` → `mdl_sel` callback flow

  **API/Type References**:
  - `~/.config/opencode/oh-my-opencode.json` — Agent/model config source

  **Acceptance Criteria**:
  - [ ] `/model` command shows paginated model list
  - [ ] Page navigation works (prev/next buttons)
  - [ ] Clicking model selects it and confirms
  - [ ] Selected model persists in state
  - [ ] `go test ./...` passes

  **Commit**: YES
  - Message: `feat(bridge): add inline keyboard model selection with pagination`
  - Files: `internal/bridge/model.go`, `internal/bridge/model_test.go`, `internal/bridge/bridge.go`, `internal/state/state.go`
  - Pre-commit: `go test ./...`

---

- [ ] 13. Reaction Tracking

  **What to do**:
  - In `internal/telegram/bot.go`:
    - Add `RegisterReactionHandler(handler ReactionHandler)` method
    - `ReactionHandler func(ctx context.Context, messageID int, emoji string, userID int64)`
    - Use `bot.RegisterHandlerMatchFunc` to match updates where `update.MessageReaction != nil`
  - In `internal/bridge/bridge.go`:
    - Register reaction handler in `RegisterHandlers()`
    - When user reacts to a bot message:
      - Log the reaction: `[REACTION] User reacted with {emoji} to message {id}`
      - Optionally: send reaction info to OpenCode as a system message in current session
      - Format: `"[User reacted with 👍 to your previous response]"`
  - Handle edge cases:
    - Bot needs admin rights to see reactions in groups (but we're DM only, should work)
    - Multiple reactions: track latest only

  **Must NOT do**:
  - Do not send reaction notifications if they'd interrupt an ongoing AI response
  - Do not crash on missing reaction data

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 12, 14)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:112-133` — RegisterTextHandler pattern (adapt for reactions)
  - go-telegram/bot: `update.MessageReaction` field

  **API/Type References**:
  - Telegram Bot API: MessageReactionUpdated type
  - go-telegram/bot models: `models.MessageReactionUpdated`

  **Acceptance Criteria**:
  - [ ] Reaction handler registered
  - [ ] Reaction logged when user reacts to bot message
  - [ ] Reaction info sent to OpenCode session
  - [ ] `go test ./...` passes

  **Commit**: YES (group with Task 14)
  - Message: `feat(telegram): add reaction tracking and agent notification`
  - Files: `internal/telegram/bot.go`, `internal/bridge/bridge.go`
  - Pre-commit: `go test ./...`

---

- [ ] 14. Sticker Search & Send

  **What to do**:
  - Create `internal/telegram/sticker.go`:
    - `StickerCache` struct: in-memory cache of sticker set names → sticker file IDs
    - `SearchSticker(bot, query string) (*models.Sticker, error)`: search installed sticker packs
    - `SendSticker(ctx, bot, chatID, fileID string) error`: send sticker
  - In `internal/telegram/bot.go`:
    - Add `SendSticker(ctx context.Context, fileID string) (int, error)` method
    - Add `RegisterStickerHandler(handler StickerHandler)` for incoming stickers
    - `StickerHandler func(ctx context.Context, sticker *models.Sticker)`
  - In `internal/bridge/bridge.go`:
    - Register sticker handler
    - When user sends sticker: extract emoji, send as text message to OpenCode: `"[Sticker: {emoji}]"`
    - When AI response contains sticker-like content (optional, low priority): could send sticker back

  **Must NOT do**:
  - Do not download sticker files
  - Do not support animated/video stickers (detect and skip)
  - Do not build a complex sticker search — basic emoji matching only

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 12, 13)
  - **Blocks**: Task 19
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:112-133` — Handler registration pattern

  **API/Type References**:
  - go-telegram/bot: `models.Sticker`, `bot.SendStickerParams`
  - Telegram Bot API: sendSticker, getStickerSet

  **Acceptance Criteria**:
  - [ ] Sticker handler registered
  - [ ] Incoming sticker converted to emoji text for AI
  - [ ] SendSticker method works
  - [ ] Animated/video stickers gracefully skipped
  - [ ] `go test ./...` passes

  **Commit**: YES (group with Task 13)
  - Message: `feat(telegram): add sticker handling and send support`
  - Files: `internal/telegram/sticker.go`, `internal/telegram/sticker_test.go`, `internal/telegram/bot.go`, `internal/bridge/bridge.go`
  - Pre-commit: `go test ./...`

---

- [ ] 15. Webhook Mode (Alternative to Polling)

  **What to do**:
  - In `internal/telegram/bot.go`:
    - Add `StartWebhook(ctx context.Context, listenAddr, webhookURL, secretToken string) error` method
    - Uses go-telegram/bot's webhook support: `bot.SetWebhook` + HTTP handler
    - Secret token for webhook verification
  - In `cmd/main.go`:
    - Add env vars: `TELEGRAM_WEBHOOK_URL`, `TELEGRAM_WEBHOOK_PORT`, `TELEGRAM_WEBHOOK_SECRET`
    - If `TELEGRAM_WEBHOOK_URL` is set: use webhook mode
    - If not set: use polling (default, current behavior)
    - Log which mode is active on startup
  - Add `TELEGRAM_MODE` env var: `polling` (default) or `webhook`

  **Must NOT do**:
  - Do not break existing polling mode
  - Do not require webhook for local development
  - Do not handle TLS termination (assume reverse proxy like nginx/Caddy)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Task 16)
  - **Blocks**: Task 17
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:104-106` — Current Start() with polling
  - go-telegram/bot webhook: `bot.SetWebhook`, `bot.StartWebhook`

  **External References**:
  - Telegram webhook API: https://core.telegram.org/bots/api#setwebhook
  - go-telegram/bot webhook docs

  **Acceptance Criteria**:
  - [ ] Webhook mode works when TELEGRAM_WEBHOOK_URL is set
  - [ ] Polling mode still works as default
  - [ ] Secret token verification enabled
  - [ ] Startup log shows active mode
  - [ ] `go test ./...` passes

  **Commit**: YES
  - Message: `feat(telegram): add webhook mode as alternative to long polling`
  - Files: `internal/telegram/bot.go`, `cmd/main.go`
  - Pre-commit: `go test ./...`

---

- [ ] 16. Proxy Support (SOCKS/HTTP)

  **What to do**:
  - In `cmd/main.go`:
    - Add `TELEGRAM_PROXY` env var (e.g., `socks5://127.0.0.1:1080` or `http://proxy:8080`)
  - In `internal/telegram/bot.go`:
    - Add `WithProxy(proxyURL string)` option
    - Parse proxy URL, create `http.Transport` with proxy
    - Pass custom HTTP client to go-telegram/bot via `bot.WithHTTPClient()`
  - Support:
    - `socks5://host:port` — SOCKS5 proxy
    - `http://host:port` — HTTP proxy
    - `https://host:port` — HTTPS proxy
  - Use `golang.org/x/net/proxy` for SOCKS5 or `net/url` + `http.Transport.Proxy` for HTTP

  **Must NOT do**:
  - Do not require proxy (optional)
  - Do not proxy OpenCode connections (only Telegram API)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Task 15)
  - **Blocks**: Task 17
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/telegram/bot.go:17-33` — NewBot constructor (add proxy option)
  - go-telegram/bot: `bot.WithHTTPClient()` option

  **External References**:
  - golang.org/x/net/proxy for SOCKS5
  - OpenClaw proxy: `proxy.ts` in their Telegram implementation

  **Acceptance Criteria**:
  - [ ] SOCKS5 proxy URL parsed correctly
  - [ ] HTTP proxy URL parsed correctly
  - [ ] Bot uses proxy for all Telegram API calls
  - [ ] No proxy = direct connection (default)
  - [ ] `go build -o opencode-telegram ./cmd` succeeds (may need `go get golang.org/x/net`)

  **Commit**: YES (group with Task 15)
  - Message: `feat(telegram): add SOCKS5/HTTP proxy support`
  - Files: `internal/telegram/bot.go`, `cmd/main.go`, `go.mod`, `go.sum`
  - Pre-commit: `go test ./...`

---

- [ ] 17. Multi-Account Bot Support

  **What to do**:
  - Refactor `cmd/main.go`:
    - Support multiple bot configurations via `TELEGRAM_BOTS` env var (JSON array):
      ```json
      [{"token":"TOKEN1","chatID":12345},{"token":"TOKEN2","chatID":67890}]
      ```
    - If `TELEGRAM_BOTS` not set: fall back to single `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID` (backward compatible)
    - For each bot config: create separate Bot instance, Bridge instance, SSE consumer
    - All share the same OpenCode client
  - In `internal/telegram/bot.go`:
    - No changes needed (already per-instance)
  - In `internal/bridge/bridge.go`:
    - No changes needed (already per-instance)
  - In `internal/state/state.go`:
    - Each bot gets its own AppState instance (already per-instance)
  - Startup: launch all bots concurrently

  **Must NOT do**:
  - Do not break single-bot mode (backward compatible)
  - Do not share state between bots (each has own session, agent, etc.)
  - Do not support more than 10 bots (reasonable limit)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 15, 16)
  - **Parallel Group**: Sequential after Wave 5
  - **Blocks**: Task 18
  - **Blocked By**: Tasks 15, 16

  **References**:

  **Pattern References**:
  - `cmd/main.go:1-91` — Current single-bot initialization (refactor to loop)
  - OpenClaw multi-account: accounts.ts resolves multiple tokens per channel

  **Acceptance Criteria**:
  - [ ] Single bot mode still works (backward compatible)
  - [ ] `TELEGRAM_BOTS` env var with JSON array creates multiple bots
  - [ ] Each bot has independent state
  - [ ] All bots start concurrently
  - [ ] `go test ./...` passes

  **Commit**: YES
  - Message: `feat(multi-account): support multiple Telegram bot instances`
  - Files: `cmd/main.go`
  - Pre-commit: `go test ./...`

---

- [ ] 18. Multi-Agent Routing

  **What to do**:
  - In `internal/state/state.go`:
    - Add `agentRoutes map[int64]string` — maps chatID → agentName
    - Add `GetAgentForChat(chatID int64) string` / `SetAgentRoute(chatID int64, agent string)`
    - Default: returns `currentAgent` if no specific route
  - In `internal/bridge/bridge.go`:
    - Modify message handler to check agent route for the sender's chat ID
    - Use route-specific agent when sending prompts
  - In `internal/bridge/commands.go`:
    - Add `/route <agent>` command to set the agent for current chat
    - `/routes` command to list all routes
  - Config file support (optional): `~/.opencode-telegram-routes.json`
    - Load on startup, save when routes change

  **Must NOT do**:
  - Do not add per-group routing (groups not supported)
  - Do not override manual `/switch` commands (explicit switch takes priority)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 6)
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 1, 17

  **References**:

  **Pattern References**:
  - `internal/state/state.go:1-65` — AppState pattern (extend with routes)
  - `internal/bridge/agent.go` — Agent switching pattern
  - OpenClaw: resolveAgentRoute() in their routing system

  **Acceptance Criteria**:
  - [ ] `/route sisyphus` sets agent for current chat
  - [ ] `/routes` shows all configured routes
  - [ ] Messages routed to correct agent based on chat ID
  - [ ] Manual `/switch` overrides route
  - [ ] `go test ./...` passes

  **Commit**: YES
  - Message: `feat(routing): add per-chat multi-agent routing`
  - Files: `internal/state/state.go`, `internal/bridge/bridge.go`, `internal/bridge/commands.go`
  - Pre-commit: `go test ./...`

---

- [ ] 19. Final Integration Test + Rebuild + Restart

  **What to do**:
  - Run full test suite: `go test ./... -count=1 -race`
  - Run vet: `go vet ./...`
  - Build final binary: `go build -o opencode-telegram ./cmd`
  - Verify binary size is reasonable (should be ~10-15MB with new features)
  - Restart LaunchAgent:
    - `launchctl unload ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist`
    - `launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist`
  - Verify service starts: `launchctl list | grep opencode`
  - Monitor logs: `tail -f ~/Library/Logs/opencode-telegram/stdout.log`
  - Send test message via Telegram
  - Verify HTML formatting in response
  - Send test image via Telegram
  - Verify vision analysis response

  **Must NOT do**:
  - Do not skip race detection tests
  - Do not leave service in broken state

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: [`playwright`]
    - `playwright`: For Telegram web verification if needed

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 6, final)
  - **Blocks**: None (final task)
  - **Blocked By**: ALL previous tasks

  **References**:

  **Pattern References**:
  - `~/Library/LaunchAgents/com.opencode.telegram-bridge.plist` — LaunchAgent config
  - `~/Library/Logs/opencode-telegram/stdout.log` — Log file

  **Acceptance Criteria**:
  - [ ] `go test ./... -count=1 -race` — all tests pass, zero race conditions
  - [ ] `go vet ./...` — no issues
  - [ ] `go build -o opencode-telegram ./cmd` — builds successfully
  - [ ] Service starts and stays running (PID visible in `launchctl list`)
  - [ ] Text message → HTML-formatted response in Telegram
  - [ ] Image message → vision analysis response
  - [ ] Streaming updates visible (message progressively updates)
  - [ ] Typing indicator shown during processing
  - [ ] All commands respond: /help, /status, /sessions, /switch, /model

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Full build and service restart
    Tool: Bash
    Preconditions: All previous tasks complete
    Steps:
      1. go test ./... -count=1 -race
      2. Assert: all tests pass, 0 race conditions
      3. go vet ./...
      4. Assert: no issues
      5. go build -o opencode-telegram ./cmd
      6. Assert: binary exists, size 8-15MB
      7. launchctl unload ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
      8. sleep 2
      9. launchctl load ~/Library/LaunchAgents/com.opencode.telegram-bridge.plist
      10. sleep 3
      11. launchctl list | grep opencode
      12. Assert: service running (exit code 0 or PID shown)
      13. tail -5 ~/Library/Logs/opencode-telegram/stdout.log
      14. Assert: no error messages in recent logs
    Expected Result: Clean build, all tests pass, service running
    Evidence: Full terminal output captured

  Scenario: End-to-end message test
    Tool: Bash (curl Telegram API)
    Preconditions: Service running, bot token known
    Steps:
      1. curl -s "https://api.telegram.org/bot${TOKEN}/sendMessage" \
           -d "chat_id=${CHAT_ID}" -d "text=Integration test: hello"
      2. sleep 15
      3. tail -20 ~/Library/Logs/opencode-telegram/stdout.log
      4. Assert: logs show message received, prompt sent, response delivered
      5. Assert: no "parse entities" errors (HTML mode working)
    Expected Result: Full round-trip message works with HTML formatting
    Evidence: Log output captured
  ```

  **Commit**: YES
  - Message: `chore: final integration verification and v2 release build`
  - Files: none (build artifact only)
  - Pre-commit: `go test ./... -count=1 -race && go vet ./...`

---

## Commit Strategy

| After Task | Message | Key Files | Verification |
|------------|---------|-----------|--------------|
| 1 | `fix(bridge): wire all command handlers, agent switching, and remove debug logs` | bridge.go, commands.go, agent.go, bot.go, main.go | go test ./... |
| 2 | `feat(telegram): replace MarkdownV2 with HTML rendering` | format.go, bot.go | go test ./internal/telegram/... |
| 3 | `refactor(opencode): remove hardcoded system prompt and unused server password` | client.go, types.go, main.go | go test ./... |
| 4 | `feat(bridge): add streaming replies via SSE message.part.updated` | bridge.go | go test ./internal/bridge/... |
| 5 | `feat(media): add image download and vision analysis pipeline` | media.go, types.go, client.go, bridge.go, bot.go | go test ./... |
| 6+7 | `feat(telegram): add typing indicators and update offset persistence` | bot.go, bridge.go, offset.go, main.go | go test ./... |
| 8 | `feat(bridge): implement question handler with inline keyboard` | question.go, bridge.go, keyboard.go | go test ./... |
| 9 | `feat(bridge): add message fragment reassembly` | bridge.go | go test ./internal/bridge/... |
| 10 | `feat(bridge): add inbound message debouncing` | bridge.go, main.go | go test ./internal/bridge/... |
| 11 | `fix(state): start IDRegistry TTL cleanup goroutine` | registry.go, main.go | go test ./internal/state/... |
| 12 | `feat(bridge): add inline keyboard model selection with pagination` | model.go, bridge.go, state.go | go test ./... |
| 13+14 | `feat(telegram): add reaction tracking and sticker support` | bot.go, bridge.go, sticker.go | go test ./... |
| 15+16 | `feat(telegram): add webhook mode and proxy support` | bot.go, main.go, go.mod | go test ./... |
| 17 | `feat(multi-account): support multiple Telegram bot instances` | main.go | go test ./... |
| 18 | `feat(routing): add per-chat multi-agent routing` | state.go, bridge.go, commands.go | go test ./... |
| 19 | `chore: final integration verification and v2 release build` | — | go test ./... -race && go vet |

---

## Success Criteria

### Verification Commands
```bash
go test ./... -count=1 -race    # All tests pass, 0 race conditions
go vet ./...                     # No issues
go build -o opencode-telegram ./cmd  # Binary builds
launchctl list | grep opencode   # Service running
```

### Final Checklist
- [ ] All "Must Have" features present and tested
- [ ] All "Must NOT Have" guardrails respected
- [ ] All 19 tasks completed with commits
- [ ] Binary builds and service runs
- [ ] HTML-formatted responses in Telegram
- [ ] Streaming replies work
- [ ] Image vision analysis works
- [ ] All commands respond
- [ ] No debug logs in production code
- [ ] Existing 74+ tests still pass
