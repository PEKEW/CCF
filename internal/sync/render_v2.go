package sync

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PEKEW/CCF/internal/session"
	"github.com/PEKEW/CCF/internal/templates"
)

// This file renders the v2 (human-surface) document set. Every renderer takes
// ONLY *session.SessionState — never []Event — so the raw event log can never
// leak into Feishu.

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

func healthIcon(h string) string {
	switch h {
	case "green":
		return "🟢 green"
	case "yellow":
		return "🟡 yellow"
	case "red":
		return "🔴 red"
	default:
		return "⚪ unknown"
	}
}

func goalOf(st *session.SessionState) string {
	if st.Contract.Goal != "" {
		return st.Contract.Goal
	}
	if st.Title != "" && st.Title != "Untitled" {
		return st.Title
	}
	return "(goal not set)"
}

// RenderCockpit builds 00_COCKPIT (replace). One-glance driving panel.
func RenderCockpit(st *session.SessionState) string {
	var b strings.Builder
	b.WriteString("# Cockpit\n\n")
	fmt.Fprintf(&b, "**%s**\n\n", goalOf(st))

	progress := st.Cockpit.ProgressNote
	if progress == "" {
		progress = criteriaProgress(st)
	}
	fmt.Fprintf(&b, "Status: phase `%s` · progress %s · health %s\n\n",
		dash(st.Phase), dash(progress), healthIcon(st.Cockpit.Health))

	fmt.Fprintf(&b, "## Now\n\n%s\n\n", dash(st.Cockpit.Summary))

	blocker := st.Cockpit.Blocker
	if blocker == "" {
		blocker = "none"
	}
	fmt.Fprintf(&b, "## Blocker\n\n%s\n\n", blocker)
	fmt.Fprintf(&b, "## Next Step\n\n%s\n\n", dash(st.Cockpit.NextStep))

	b.WriteString("## Key Links\n\n")
	link := func(name string, k templates.DocKey) {
		if d, ok := st.Docs[string(k)]; ok {
			fmt.Fprintf(&b, "- %s: %s\n", name, dash(d.URL))
		} else {
			fmt.Fprintf(&b, "- %s:\n", name)
		}
	}
	link("Task Contract", templates.KeyContract)
	link("Recap", templates.KeyRecap)
	link("Validation & Decisions", templates.KeyValDecs)
	link("Handoff", templates.KeyHandoff)
	link("Memory", templates.KeyMemory)
	link("Memo", templates.KeyMemo)

	b.WriteString("\n---\n\n## Metadata\n\n")
	fmt.Fprintf(&b, "- Session ID: %s\n", dash(st.SessionID))
	fmt.Fprintf(&b, "- Status: %s\n", dash(st.Status))
	fmt.Fprintf(&b, "- Created: %s\n", ts(st.CreatedAt))
	fmt.Fprintf(&b, "- Last Sync: %s\n", ts(st.LastSyncAt))
	fmt.Fprintf(&b, "- Feishu Folder: %s\n", dash(st.FeishuFolderURL))
	fmt.Fprintf(&b, "- Local CWD: %s\n", dash(st.CWD))
	fmt.Fprintf(&b, "- Git: branch=%s commit=%s dirty=%t\n",
		dash(st.GitBranch), dash(st.GitCommit), st.GitDirty)
	return b.String()
}

func criteriaProgress(st *session.SessionState) string {
	c := st.Contract.AcceptanceCriteria
	if len(c) == 0 {
		return "-"
	}
	done := 0
	for _, x := range c {
		if x.Done {
			done++
		}
	}
	return fmt.Sprintf("%d/%d criteria", done, len(c))
}

// RenderContract builds 01_TASK_CONTRACT (replace).
func RenderContract(st *session.SessionState) string {
	c := st.Contract
	var b strings.Builder
	b.WriteString("# Task Contract\n\n")
	fmt.Fprintf(&b, "## Goal\n\n%s\n\n", dash(c.Goal))
	fmt.Fprintf(&b, "## Why\n\n%s\n\n", dash(c.Why))
	writeList(&b, "In Scope", c.InScope)
	writeList(&b, "Out of Scope", c.OutScope)

	b.WriteString("## Acceptance Criteria\n\n")
	if len(c.AcceptanceCriteria) == 0 {
		b.WriteString("- (none yet)\n\n")
	} else {
		for _, cr := range c.AcceptanceCriteria {
			mark := " "
			if cr.Done {
				mark = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", mark, cr.Text)
		}
		b.WriteString("\n")
	}

	writeList(&b, "Constraints", c.Constraints)
	writeList(&b, "Known Risks", c.Risks)
	if !c.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "_Updated: %s_\n", ts(c.UpdatedAt))
	}
	return b.String()
}

// RenderRecap builds 02_RECAP (replace) — Claude-authored prose verbatim.
func RenderRecap(st *session.SessionState) string {
	var b strings.Builder
	b.WriteString("# Recap\n\n")
	if strings.TrimSpace(st.RecapNarrative) == "" {
		b.WriteString("_(no recap written yet)_\n")
	} else {
		b.WriteString(st.RecapNarrative)
		if !strings.HasSuffix(st.RecapNarrative, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderHandoffV2 builds 04_HANDOFF (replace) from structured state.
func RenderHandoffV2(st *session.SessionState) string {
	h := st.Handoff
	var b strings.Builder
	b.WriteString("# Handoff\n\n")
	writeList(&b, "What This Session Tried To Do", h.Tried)
	writeList(&b, "What Has Been Completed", h.Done)
	writeList(&b, "What Remains", h.Remains)
	writeList(&b, "Known Risks", h.Risks)

	fmt.Fprintf(&b, "## Current State\n\nphase=%s status=%s branch=%s commit=%s dirty=%t\n\n",
		dash(st.Phase), dash(st.Status), dash(st.GitBranch), dash(st.GitCommit), st.GitDirty)

	if len(h.HowToResume) == 0 {
		b.WriteString("## How To Resume\n\n")
		fmt.Fprintf(&b, "- cd %s\n", dash(st.CWD))
		if st.GitBranch != "" {
			fmt.Fprintf(&b, "- git checkout %s\n", st.GitBranch)
		}
		b.WriteString("\n")
	} else {
		writeList(&b, "How To Resume", h.HowToResume)
	}
	writeList(&b, "Files To Read First", h.FilesToRead)
	return b.String()
}

// RenderMemory builds 05_MEMORY (replace-render of the full slice, grouped by
// kind). Append is implemented as state-append + full re-render to avoid drift.
func RenderMemory(st *session.SessionState) string {
	var b strings.Builder
	b.WriteString("# Memory\n\n")
	if len(st.Memory) == 0 {
		b.WriteString("_(empty)_\n")
		return b.String()
	}
	order := []string{"important", "gotcha", "constraint", "decision", "resource", "fact"}
	titles := map[string]string{
		"important":  "重要部分 Important",
		"gotcha":     "注意事项 Gotchas",
		"constraint": "约束 Constraints",
		"decision":   "决定 Decisions",
		"resource":   "资源 Resources",
		"fact":       "其他 Facts",
	}
	byKind := map[string][]session.MemoryItem{}
	var extra []string
	for _, m := range st.Memory {
		k := m.Kind
		if _, ok := titles[k]; !ok {
			k = "fact"
		}
		byKind[k] = append(byKind[k], m)
	}
	for k := range byKind {
		if _, known := titles[k]; !known {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range order {
		items := byKind[k]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", titles[k])
		for _, m := range items {
			fmt.Fprintf(&b, "- %s\n", m.Text)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ExtractSection returns the text between start and end markers (exclusive),
// trimmed. Returns "" if either marker is missing.
func ExtractSection(text, start, end string) string {
	i := strings.Index(text, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(text[i:], end)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(text[i : i+j])
}

// MemoHumanText returns the human-authored section of a memo doc's raw text,
// stripped of the heading and placeholder. Returns "" when empty or untouched.
func MemoHumanText(raw string) string {
	body := ExtractSection(raw, templates.MemoHumanStart, templates.MemoHumanEnd)
	var keep []string
	for _, ln := range strings.Split(body, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		// Drop the placeholder line shipped in the template.
		if strings.HasPrefix(t, "(在这里写给") {
			continue
		}
		keep = append(keep, t)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

// RenderMemo rebuilds the memo doc, preserving the human section verbatim and
// replacing only Claude's section. humanBody is the raw text already between the
// HUMAN markers (pass "" to keep the template placeholder).
func RenderMemo(humanBody string, claudeNotes []string) string {
	if strings.TrimSpace(humanBody) == "" {
		humanBody = "## 人工备注 (Human notes — Claude reads this on resume)\n\n(在这里写给 Claude 的提示、约束、上下文。下次会话 Claude 会自动读到。)"
	}
	var cb strings.Builder
	cb.WriteString("## Claude 备注 (Claude's notes to you)\n\n")
	if len(claudeNotes) == 0 {
		cb.WriteString("(none yet)")
	} else {
		for _, n := range claudeNotes {
			fmt.Fprintf(&cb, "- %s\n", n)
		}
	}
	return fmt.Sprintf("# 备忘录 Memo\n\n%s\n%s\n%s\n\n%s\n%s\n%s\n",
		templates.MemoHumanStart, humanBody, templates.MemoHumanEnd,
		templates.MemoClaudeStart, strings.TrimRight(cb.String(), "\n"), templates.MemoClaudeEnd)
}

// RenderValidationEntry returns a single block to append to 03 (validation).
func RenderValidationEntry(now time.Time, line string) string {
	return fmt.Sprintf("\n### Validation %s\n\n- %s\n", now.Format(time.RFC3339), line)
}

// RenderDecisionEntry returns a single block to append to 03 (decisions).
func RenderDecisionEntry(now time.Time, verdict, ctx, rule, reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n### Decision %s\n\n", now.Format(time.RFC3339))
	if ctx != "" {
		fmt.Fprintf(&b, "- Context: %s\n", ctx)
	}
	if verdict != "" {
		fmt.Fprintf(&b, "- Verdict: %s\n", verdict)
	}
	if rule != "" {
		fmt.Fprintf(&b, "- Rule: %s\n", rule)
	}
	if reason != "" {
		fmt.Fprintf(&b, "- Reason: %s\n", reason)
	}
	return b.String()
}

func writeList(b *strings.Builder, title string, items []string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(items) == 0 {
		b.WriteString("- (none)\n\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteString("\n")
}
