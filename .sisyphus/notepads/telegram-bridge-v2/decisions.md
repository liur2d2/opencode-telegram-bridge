# Decisions — Telegram Bridge V2

Architectural choices made during implementation.

---

## [2026-02-07T14:56] Initial Decisions (from Planning)

**MarkdownV2 → HTML**:
- Abandoned ParseMode "MarkdownV2" due to AI-generated markdown conflicts with escaping rules
- Using ParseMode "HTML" instead — simpler and more reliable
- Reference: Telegram HTML spec https://core.telegram.org/bots/api#html-style

**System Prompt Handling**:
- Removing hardcoded "回覆使用繁體中文" from client.go
- Let OpenCode's own config (`~/.config/opencode/opencode.json`) handle system prompts
- Keeps System field in SendPromptRequest for future flexibility

**Media Support**:
- Images only (Telegram photo → base64 → OpenCode vision API)
- Voice/video/documents: graceful "unsupported" reply
- No persistent storage — process in memory, discard

**Group Chat**:
- NOT implementing group support (user declined)
- DM only, single chat ID per bot instance
- Skip: forum topics, @mention gating, per-group config

**Multi-Account**:
- Multiple bot instances supported (each with own token + chat ID)
- No shared state between instances


## Task 3: Remove Hardcoded System Prompt and Dead ServerPassword Config (2025-02-07)

### Changes Made
1. **internal/opencode/client.go:181-191** - SendPrompt() method
   - Removed hardcoded systemPrompt assignment: `systemPrompt := "回覆使用繁體中文"`
   - Changed System field from `&systemPrompt` to `nil`
   - Rationale: OpenCode's own config (~/.config/opencode/opencode.json) handles system prompts

2. **internal/opencode/types.go:6-9** - Config struct
   - Removed `ServerPassword string` field
   - Kept only `BaseURL` and `Directory` fields
   - Rationale: Dead code, no authentication implemented in OpenCode API

3. **cmd/main.go:19-42** - main() function
   - Removed `ocPassword := os.Getenv("OPENCODE_SERVER_PASSWORD")` line 23
   - Removed `ServerPassword: ocPassword,` from Config initialization
   - Rationale: No longer needed with ServerPassword field removal

### Preserved Functionality
- **System field retained** in SendPromptRequest (types.go:105)
  - Kept as `*string` with JSON tag `"system,omitempty"`
  - Enables future per-message system prompt overrides if needed
  - Currently unused (System: nil), but available for future features

### Test Results
- All 40 tests passed (8.94s duration)
  - bridge: 32 tests passed
  - opencode: 9 tests passed
  - state: 6 tests passed
  - telegram: 13 tests passed
- No tests required updating (no tests referenced ServerPassword)

### Verification
✓ grep -rn '回覆使用繁體中文' returns 0 matches
✓ grep -rn 'ServerPassword' returns 0 matches
✓ System *string field still exists in SendPromptRequest
✓ go test ./... -count=1 passes (all tests)
✓ Zero hardcoded system prompts sent to OpenCode

### Decision Rationale
- System prompts now controlled externally via OpenCode's own configuration
- Telegram bridge no longer forces Chinese language requirement
- Cleaner separation of concerns: Telegram bridge handles routing, OpenCode handles AI configuration
- ServerPassword was never used and represents technical debt
