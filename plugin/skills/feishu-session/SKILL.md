---
name: feishu-session
description: Maintain the Feishu session docs (cockpit, contract, recap, decisions, handoff, memory) for a ccfl-managed Claude Code session. Use whenever working in a session where the ccfl hooks/MCP server are active — to keep the Feishu project docs current as a human surface, not an event log.
---

# Feishu Session Docs

A ccfl-managed session mirrors your work into six Feishu documents that are a
**human surface** — a project cockpit, a task contract, and a handoff — NOT an
agent event log. ccfl handles the machine parts (folder, title, syncing, blocking
dangerous ops). YOU author the narrative via the `mcp__ccfl__*` tools.

Never paste raw tool output or event lists into these docs. Write concise prose
a human stakeholder would want to read.

## The six docs

| Doc | What it holds | Tool |
|---|---|---|
| 00 Cockpit | One-glance status: goal, health, what's happening now, blocker, next step | `feishu_update_cockpit` |
| 01 Task Contract | Goal, scope, acceptance criteria, constraints, risks | `feishu_set_contract` |
| 02 Recap | Narrative of progress + key decisions (prose) | `feishu_update_recap` |
| 03 Validation & Decisions | Test results + meaningful decisions (append) | `feishu_append_validation`, `feishu_append_decision` |
| 04 Handoff | Tried / done / remains / risks / how to resume | `feishu_update_handoff` |
| 05 Memory | Durable facts that must survive compaction | `feishu_append_memory` |

## When to call each tool

- **`feishu_set_contract`** — once after the first prompt when you have an initial
  plan; update when scope changes. Always express acceptance criteria as a
  checklist (`done: true/false`). This is the agreement; keep it honest.
- **`feishu_update_cockpit`** — on every phase change, when a blocker appears or
  clears (pass `blocker: ""` to clear), and at the start/end of a focused work
  chunk. Keep `summary` to one or two sentences. Set `health` (green/yellow/red).
- **`feishu_append_decision`** — at each meaningful choice (library, approach,
  tradeoff) with a one-line rationale. Skip trivial mechanics.
- **`feishu_append_validation`** — after running tests/build/lint. Record in human
  terms, e.g. `go test ./... → PASS 142 tests`.
- **`feishu_update_recap`** — periodically (every few chunks) and before you stop.
  Narrative: what's done, key decisions, current focus. No event list.
- **`feishu_append_memory`** — durable facts that must outlive a compaction:
  constraints, gotchas, external resources, firm decisions. Call **before** you
  expect context to compact.
- **`feishu_update_handoff`** — at Stop / end of session: what you tried, what's
  done, what remains, risks, how to resume, which files to read first.
- **`feishu_get_status`** — read current goal/phase/health/criteria when resuming
  or when unsure of state.

All tools accept an optional `session_id`; omit it to target the active session.

## Protocol discipline

- **Stop digging.** If the same failure repeats, the scope expands, or a risky
  operation is required: stop, set the cockpit blocker + health `red`, and ask the
  user. Do not loop. ccfl's contract template lists these stop conditions.
- **Test integrity.** Never weaken tests, golden outputs, thresholds, CI, or
  lockfiles to make things pass. ccfl's PreToolUse policy will block/ask on these
  anyway. If a test genuinely must change, record it via `feishu_append_decision`
  with rationale and get user approval first.
- **Don't duplicate ccfl's automatic work.** ccfl already records policy
  blocks/approvals into Validation & Decisions and auto-distills Memory at
  PreCompact. Add only what it can't infer: your reasoning and narrative.
