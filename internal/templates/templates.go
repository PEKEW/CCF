package templates

// DocKey identifies one of the core session documents.
type DocKey string

const (
	KeyIndex          DocKey = "00_SESSION_INDEX"
	KeyTaskPlan       DocKey = "01_TASK_AND_PLAN"
	KeyActiveContext  DocKey = "02_ACTIVE_CONTEXT"
	KeyCheckpoints    DocKey = "03_CHECKPOINTS"
	KeyValidationDecs DocKey = "04_VALIDATION_AND_DECISIONS"
	KeyHandoff        DocKey = "05_HANDOFF"
)

// CoreDocs is the ordered list of MVP documents.
var CoreDocs = []DocKey{
	KeyIndex, KeyTaskPlan, KeyActiveContext, KeyCheckpoints, KeyValidationDecs, KeyHandoff,
}

// Initial returns the starter content for a document key.
func Initial(k DocKey) string {
	switch k {
	case KeyIndex:
		return indexTmpl
	case KeyTaskPlan:
		return taskPlanTmpl
	case KeyActiveContext:
		return activeContextTmpl
	case KeyCheckpoints:
		return checkpointsTmpl
	case KeyValidationDecs:
		return validationTmpl
	case KeyHandoff:
		return handoffTmpl
	}
	return ""
}

const indexTmpl = `# Session Index

## Basic Info

- Session ID:
- Title:
- Status:
- Phase:
- Created:
- Last Sync:
- Feishu Folder:
- Local CWD:
- Git Branch:
- Git Commit:

## Current Summary

## Current Blocker

## Next Step

## Key Links

- Task and Plan:
- Active Context:
- Checkpoints:
- Validation and Decisions:
- Handoff:
`

const taskPlanTmpl = `# Task and Plan

## Goal

## Non-goals

## Assumptions

## Constraints

## Plan

## Expected Output of Each Step

## Validation Method

## Stop Conditions

Stop and ask the user when:

- The same failure repeats.
- The task scope expands.
- A risky operation is required.
- Tests, thresholds, golden outputs, CI, lockfiles, or deployment settings need changes.
- The next step is not clearly justified.

## Human Approval Required

## Plan Revisions
`

const activeContextTmpl = `# Active Context

## Current Goal

## Current Phase

## Current Working Directory

## Git State

## Active Files

## Current Assumptions

## Recent Changes

## Validation Status

## Open Decisions

## Current Blocker

## Next Step

## What Must Not Be Forgotten
`

const checkpointsTmpl = `# Checkpoints
`

const validationTmpl = `# Validation and Decisions

## Validation Records

## Decisions
`

const handoffTmpl = `# Handoff Summary

## What This Session Tried To Do

## What Has Been Completed

## What Remains

## Current State

## Known Risks

## How To Resume

## Files To Read First

## Commands To Run

## Feishu Docs To Check
`
