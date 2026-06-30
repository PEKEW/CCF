package app

import (
	"context"
	"time"

	"github.com/PEKEW/CCF/internal/session"
	"github.com/PEKEW/CCF/internal/summary"
	syncpkg "github.com/PEKEW/CCF/internal/sync"
	"github.com/PEKEW/CCF/internal/templates"
)

const (
	summaryTimeout     = 90 * time.Second
	summaryMinInterval = 5 * time.Minute // throttle Stop-triggered summaries
	memoNoteCap        = 6
)

// generateSummary runs the headless claude -p engine to refresh the
// human-readable cockpit/recap/memory prose, then merges the result into state.
// It is throttled: unless force is set it skips if a summary ran recently, so
// the per-turn Stop hook does not spawn a claude call every turn. Best-effort —
// failures fall back to the deterministic distiller in flush().
func (a *App) generateSummary(st *session.SessionState, now time.Time, force bool) {
	if st.TranscriptPath == "" || !summary.Available() {
		return
	}
	if !force && !st.LastSummaryAt.IsZero() && now.Sub(st.LastSummaryAt) < summaryMinInterval {
		a.notef("summary: throttled (last %s ago)", now.Sub(st.LastSummaryAt).Round(time.Second))
		return
	}

	c, cancel := context.WithTimeout(context.Background(), summaryTimeout)
	defer cancel()
	res, err := summary.Generate(c, summary.Options{
		TranscriptPath: st.TranscriptPath,
		Goal:           st.Contract.Goal,
		PrevRecap:      st.RecapNarrative,
		Lang:           "zh",
	})
	if err != nil {
		a.notef("summary: %v (fallback to deterministic)", err)
		return
	}
	st.LastSummaryAt = now

	if res.Cockpit != "" {
		st.Cockpit.Summary = res.Cockpit
		syncpkg.MarkCockpit(st, now)
	}
	if res.Recap != "" {
		st.RecapNarrative = res.Recap
		syncpkg.MarkRecap(st, now)
	}
	a.mergeEngineMemory(st, res.Memory, now)
	a.notef("summary: refreshed (cockpit=%d recap=%d mem=%d)",
		len(res.Cockpit), len(res.Recap), len(res.Memory))
}

// mergeEngineMemory folds engine-produced facts into st.Memory (deduped by text)
// and rebuilds the memo's Claude-notes section from the important/gotcha facts.
func (a *App) mergeEngineMemory(st *session.SessionState, facts []summary.MemoryFact, now time.Time) {
	if len(facts) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, m := range st.Memory {
		seen[m.Text] = true
	}
	changed := false
	var notes []string
	for _, f := range facts {
		if f.Text == "" {
			continue
		}
		if !seen[f.Text] {
			seen[f.Text] = true
			st.Memory = append(st.Memory, session.MemoryItem{Time: now, Kind: f.Kind, Text: f.Text})
			changed = true
		}
		if (f.Kind == "important" || f.Kind == "gotcha") && len(notes) < memoNoteCap {
			notes = append(notes, f.Text)
		}
	}
	if changed {
		syncpkg.MarkMemory(st, now)
	}
	if len(notes) > 0 {
		st.MemoNotes = notes
	}
}

// updateMemo rewrites 06_MEMO, preserving the human-authored section read back
// from Feishu and replacing only Claude's notes section. No-op when Claude has
// no notes to write — the human section is left untouched, so we avoid a
// per-turn read+rewrite of the doc.
func (a *App) updateMemo(st *session.SessionState) error {
	if len(st.MemoNotes) == 0 {
		return nil
	}
	raw, _ := a.readDocText(st, string(templates.KeyMemo))
	human := syncpkg.ExtractSection(raw, templates.MemoHumanStart, templates.MemoHumanEnd)
	return a.updateDoc(st, string(templates.KeyMemo), syncpkg.RenderMemo(human, st.MemoNotes))
}
