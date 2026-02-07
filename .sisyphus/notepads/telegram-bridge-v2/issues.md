# Issues — Telegram Bridge V2

Problems, gotchas, and workarounds discovered during implementation.

---

## [2026-02-07T14:56] Known Issues (from Previous Work)

**Telegram Rate Limits**:
- Message edits: max ~30 per minute per chat
- Streaming implementation must throttle to 1 edit per 500ms minimum
- "message is not modified" errors should be silently ignored

**Parts Field Extraction**:
- Parts are `[]interface{}` containing `map[string]interface{}`
- Cannot extract as strings directly
- Must type assert and iterate

**Git Operations**:
- gh CLI only works with HTTPS remotes, not SSH
- Need 5-minute HTTP timeout (not 30s) for git operations

**Binary Management**:
- `go run` leaves zombie processes
- Always use compiled binary for service
- Rebuild required after code changes

