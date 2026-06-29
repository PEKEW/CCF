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
)

// CoreDocs is the ordered v2 document set created for new sessions.
// Changing this list (and Initial) is the only place the doc set is defined.
var CoreDocs = []DocKey{
	KeyCockpit, KeyContract, KeyRecap, KeyValDecs, KeyHandoff, KeyMemory,
}

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
