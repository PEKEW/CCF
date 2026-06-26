package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/peke/cc-feishu-link/internal/session"
)

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func ts(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

// RenderIndex produces the full 00_SESSION_INDEX content (replace-on-update).
func RenderIndex(st *session.SessionState) string {
	var b strings.Builder
	b.WriteString("# Session Index\n\n## Basic Info\n\n")
	fmt.Fprintf(&b, "- Session ID: %s\n", st.SessionID)
	fmt.Fprintf(&b, "- Title: %s\n", dash(st.Title))
	fmt.Fprintf(&b, "- Status: %s\n", dash(st.Status))
	fmt.Fprintf(&b, "- Phase: %s\n", dash(st.Phase))
	fmt.Fprintf(&b, "- Created: %s\n", ts(st.CreatedAt))
	fmt.Fprintf(&b, "- Last Sync: %s\n", ts(st.LastSyncAt))
	fmt.Fprintf(&b, "- Feishu Folder: %s\n", dash(st.FeishuFolderURL))
	fmt.Fprintf(&b, "- Local CWD: %s\n", dash(st.CWD))
	fmt.Fprintf(&b, "- Git Branch: %s\n", dash(st.GitBranch))
	fmt.Fprintf(&b, "- Git Commit: %s\n", dash(st.GitCommit))

	b.WriteString("\n## Current Summary\n\n")
	b.WriteString("\n## Current Blocker\n\n")
	b.WriteString("\n## Next Step\n\n")

	b.WriteString("\n## Key Links\n\n")
	link := func(name string, k string) {
		if d, ok := st.Docs[k]; ok {
			fmt.Fprintf(&b, "- %s: %s\n", name, dash(d.URL))
		} else {
			fmt.Fprintf(&b, "- %s:\n", name)
		}
	}
	link("Task and Plan", "01_TASK_AND_PLAN")
	link("Active Context", "02_ACTIVE_CONTEXT")
	link("Checkpoints", "03_CHECKPOINTS")
	link("Validation and Decisions", "04_VALIDATION_AND_DECISIONS")
	link("Handoff", "05_HANDOFF")
	return b.String()
}

// RenderActiveContext produces the full 02_ACTIVE_CONTEXT content.
func RenderActiveContext(st *session.SessionState, events []Event) string {
	var b strings.Builder
	b.WriteString("# Active Context\n\n")
	fmt.Fprintf(&b, "## Current Goal\n\n%s\n\n", dash(st.ProvisionalTitle))
	fmt.Fprintf(&b, "## Current Phase\n\n%s\n\n", dash(st.Phase))
	fmt.Fprintf(&b, "## Current Working Directory\n\n%s\n\n", dash(st.CWD))
	fmt.Fprintf(&b, "## Git State\n\nbranch=%s commit=%s dirty=%t\n\n",
		dash(st.GitBranch), dash(st.GitCommit), st.GitDirty)

	files := touchedFiles(events)
	b.WriteString("## Active Files\n\n")
	if len(files) == 0 {
		b.WriteString("- none\n\n")
	} else {
		for _, f := range files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Recent Changes\n\n")
	recent := events
	if len(recent) > 15 {
		recent = recent[len(recent)-15:]
	}
	if len(recent) == 0 {
		b.WriteString("- none\n\n")
	} else {
		for _, e := range recent {
			fmt.Fprintf(&b, "- [%s] %s\n", e.Kind, dash(e.Summary))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Current Blocker\n\n")
	if st.Phase == session.PhaseBlocked {
		b.WriteString("Session is blocked. See validation/decisions.\n\n")
	} else {
		b.WriteString("- none\n\n")
	}
	b.WriteString("## Next Step\n\n- (to be filled by Claude)\n\n")
	b.WriteString("## What Must Not Be Forgotten\n\n")
	return b.String()
}

// RenderHandoff produces the full 05_HANDOFF content.
func RenderHandoff(st *session.SessionState, events []Event) string {
	var b strings.Builder
	b.WriteString("# Handoff Summary\n\n")
	fmt.Fprintf(&b, "## What This Session Tried To Do\n\n%s\n\n", dash(st.ProvisionalTitle))
	b.WriteString("## What Has Been Completed\n\n- (to be filled by Claude)\n\n")
	b.WriteString("## What Remains\n\n- (to be filled by Claude)\n\n")
	fmt.Fprintf(&b, "## Current State\n\nphase=%s status=%s branch=%s commit=%s dirty=%t\n\n",
		dash(st.Phase), dash(st.Status), dash(st.GitBranch), dash(st.GitCommit), st.GitDirty)
	b.WriteString("## Known Risks\n\n")
	b.WriteString("## How To Resume\n\n")
	fmt.Fprintf(&b, "- cd %s\n", dash(st.CWD))
	if st.GitBranch != "" {
		fmt.Fprintf(&b, "- git checkout %s\n", st.GitBranch)
	}
	b.WriteString("\n## Files To Read First\n\n")
	for _, f := range touchedFiles(events) {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteString("\n## Commands To Run\n\n")
	b.WriteString("\n## Feishu Docs To Check\n\n")
	for _, k := range []string{"00_SESSION_INDEX", "02_ACTIVE_CONTEXT", "03_CHECKPOINTS", "04_VALIDATION_AND_DECISIONS"} {
		if d, ok := st.Docs[k]; ok {
			fmt.Fprintf(&b, "- %s: %s\n", k, dash(d.URL))
		}
	}
	return b.String()
}
