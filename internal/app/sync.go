package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/PEKEW/CCF/internal/session"
	syncpkg "github.com/PEKEW/CCF/internal/sync"
	"github.com/PEKEW/CCF/internal/templates"
	"github.com/PEKEW/CCF/internal/transcript"
)

// flush writes the session's Feishu docs from current state, then clears dirty
// state. It renders the human-surface docs purely from SessionState and NEVER
// reads the event buffer — the raw event log stays local.
func (a *App) flush(st *session.SessionState, reason string) error {
	now := time.Now()
	a.notef("sync: %s", reason)

	// Auto-populate cockpit/recap/handoff from the transcript so docs are never
	// empty even when Claude never calls the MCP tools.
	a.autoDistill(st, now)

	// Cockpit + Recap reflect current state every sync.
	if err := a.updateDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
		return err
	}
	if err := a.updateDoc(st, string(templates.KeyRecap), syncpkg.RenderRecap(st)); err != nil {
		return err
	}
	if st.Dirty.HasContractUpdate {
		if err := a.updateDoc(st, string(templates.KeyContract), syncpkg.RenderContract(st)); err != nil {
			return err
		}
	}
	if st.Dirty.HasMemoryUpdate {
		if err := a.updateDoc(st, string(templates.KeyMemory), syncpkg.RenderMemory(st)); err != nil {
			return err
		}
	}
	if st.Dirty.HasHandoffUpdate {
		if err := a.updateDoc(st, string(templates.KeyHandoff), syncpkg.RenderHandoffV2(st)); err != nil {
			return err
		}
	}
	// 03 (validation/decisions) is append-only and written at event time.
	syncpkg.Clear(st, now)
	return a.Store.Save(st)
}

// autoDistill reads the session transcript and fills cockpit/recap/handoff
// fields heuristically (deterministic, no LLM). No-op if no transcript path.
func (a *App) autoDistill(st *session.SessionState, now time.Time) {
	if st.TranscriptPath == "" {
		return
	}
	dg, _ := transcript.Distill(st.TranscriptPath)
	if len(dg.Prompts) == 0 && len(dg.EditedFiles) == 0 && dg.LastAssistantText == "" {
		return
	}

	// The claude -p engine writes the human-readable Summary/Recap. Only fill them
	// deterministically as a FALLBACK when the engine has not produced prose yet
	// (binary missing, throttled before first run, etc.) — never clobber prose.
	if strings.TrimSpace(st.Cockpit.Summary) == "" {
		switch {
		case dg.LastAssistantText != "":
			st.Cockpit.Summary = oneLine(trunc(dg.LastAssistantText, 280))
		case dg.LastUserPrompt != "":
			st.Cockpit.Summary = "Working on: " + oneLine(trunc(dg.LastUserPrompt, 200))
		}
	}
	st.Cockpit.ProgressNote = fmt.Sprintf("%d prompts · %d files · %d validations",
		len(dg.Prompts), len(dg.EditedFiles), dg.Validations)
	if st.Cockpit.Health == "" {
		st.Cockpit.Health = "green"
	}

	if strings.TrimSpace(st.RecapNarrative) == "" {
		st.RecapNarrative = buildRecap(dg)
	}

	files := dg.EditedFiles
	if len(files) > 20 {
		files = files[len(files)-20:]
	}
	st.Handoff.FilesToRead = files

	syncpkg.MarkCockpit(st, now)
	syncpkg.MarkRecap(st, now)
	syncpkg.MarkHandoff(st, now)
}

func buildRecap(dg *transcript.Digest) string {
	var b strings.Builder
	b.WriteString("_(auto-summarized from the session transcript)_\n\n## Requests\n\n")
	ps := dg.Prompts
	if len(ps) > 12 {
		ps = ps[len(ps)-12:]
	}
	if len(ps) == 0 {
		b.WriteString("- (none yet)\n")
	}
	for _, p := range ps {
		b.WriteString("- " + oneLine(trunc(p, 160)) + "\n")
	}
	b.WriteString("\n## Activity\n\n")
	if len(dg.EditedFiles) > 0 {
		shown := dg.EditedFiles
		if len(shown) > 12 {
			shown = shown[len(shown)-12:]
		}
		fmt.Fprintf(&b, "- Edited %d file(s): %s\n", len(dg.EditedFiles), strings.Join(shown, ", "))
	}
	if dg.Validations > 0 {
		fmt.Fprintf(&b, "- Ran %d validation command(s)\n", dg.Validations)
	}
	if len(dg.EditedFiles) == 0 && dg.Validations == 0 {
		b.WriteString("- (no file edits or validations yet)\n")
	}
	return b.String()
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// maybeSync evaluates sync policy for a trigger event and flushes if needed.
func (a *App) maybeSync(st *session.SessionState, triggerEvent string) error {
	d := syncpkg.Evaluate(st, time.Now(), triggerEvent)
	if !d.Sync {
		a.notef("no sync: %s", d.Reason)
		return a.Store.Save(st)
	}
	return a.flush(st, d.Reason)
}
