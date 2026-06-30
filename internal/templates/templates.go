package templates

// DocKey identifies one of the core session documents.
type DocKey string

// v2 document set: a human surface (cockpit / contract / handoff), not an
// event log. New sessions use these.
const (
	KeyCockpit  DocKey = "00_COCKPIT"
	KeyContract DocKey = "01_TASK_CONTRACT"
	KeyRecap    DocKey = "02_RECAP"
	KeyValDecs  DocKey = "03_VALIDATION_AND_DECISIONS"
	KeyHandoff  DocKey = "04_HANDOFF"
	KeyMemory   DocKey = "05_MEMORY"
	KeyMemo     DocKey = "06_MEMO"
)

// CoreDocs is the ordered v2 document set created for new sessions.
// Changing this list (and Initial) is the only place the doc set is defined.
var CoreDocs = []DocKey{
	KeyCockpit, KeyContract, KeyRecap, KeyValDecs, KeyHandoff, KeyMemory, KeyMemo,
}

// Memo section markers. The HUMAN section is owned by the user (read back into
// Claude on resume); the CLAUDE section is written by ccfl/Claude. ccfl must
// preserve whatever sits between the HUMAN markers when it rewrites the memo.
const (
	MemoHumanStart  = "<!-- HUMAN:START -->"
	MemoHumanEnd    = "<!-- HUMAN:END -->"
	MemoClaudeStart = "<!-- CLAUDE:START -->"
	MemoClaudeEnd   = "<!-- CLAUDE:END -->"
)

// Initial returns the starter content for a v2 document key. Content is a thin
// scaffold; Claude fills the substance via the MCP tools and ccfl re-renders.
// Affects NEW sessions only — existing sessions keep their on-disk docs.
func Initial(k DocKey) string {
	switch k {
	case KeyCockpit:
		return cockpitTmpl
	case KeyContract:
		return contractTmpl
	case KeyRecap:
		return recapTmpl
	case KeyValDecs:
		return valDecsTmpl
	case KeyHandoff:
		return handoffTmpl
	case KeyMemory:
		return memoryTmpl
	case KeyMemo:
		return MemoTmpl
	}
	return ""
}

const cockpitTmpl = `# Cockpit

_(driving panel — refreshed automatically on each sync)_
`

const contractTmpl = `# Task Contract

## Goal

## Why

## In Scope

## Out of Scope

## Acceptance Criteria

## Constraints

These must NOT be broken without explicit human approval:

- Tests, golden outputs, thresholds, CI, lockfiles, deployment settings.

## Known Risks
`

const recapTmpl = `# Recap

_(narrative progress summary, authored by Claude — not an event log)_
`

const valDecsTmpl = `# Validation & Decisions

## Validation

## Decisions
`

const handoffTmpl = `# Handoff

## What This Session Tried To Do

## What Has Been Completed

## What Remains

## Known Risks

## How To Resume

## Files To Read First
`

const memoryTmpl = `# Memory

_(durable facts that must survive compaction: constraints, decisions, gotchas, resources)_
`

// MemoTmpl is the initial 06_MEMO doc: a shared notebook between the human and
// Claude. The human edits the HUMAN section in Feishu; Claude reads it on the
// next resume. ccfl preserves the HUMAN section whenever it rewrites the memo.
const MemoTmpl = `# 备忘录 Memo

` + MemoHumanStart + `
## 人工备注 (Human notes — Claude reads this on resume)

(在这里写给 Claude 的提示、约束、上下文。下次会话 Claude 会自动读到。)
` + MemoHumanEnd + `

` + MemoClaudeStart + `
## Claude 备注 (Claude's notes to you)

(none yet)
` + MemoClaudeEnd + `
`
