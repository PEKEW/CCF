package app

import (
	"time"

	"github.com/PEKEW/CCF/internal/session"
	syncpkg "github.com/PEKEW/CCF/internal/sync"
	"github.com/PEKEW/CCF/internal/templates"
)

// flush writes the session's Feishu docs from current state, then clears dirty
// state. It renders the human-surface docs purely from SessionState and NEVER
// reads the event buffer — the raw event log stays local.
func (a *App) flush(st *session.SessionState, reason string) error {
	now := time.Now()
	a.notef("sync: %s", reason)

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

// maybeSync evaluates sync policy for a trigger event and flushes if needed.
func (a *App) maybeSync(st *session.SessionState, triggerEvent string) error {
	d := syncpkg.Evaluate(st, time.Now(), triggerEvent)
	if !d.Sync {
		a.notef("no sync: %s", d.Reason)
		return a.Store.Save(st)
	}
	return a.flush(st, d.Reason)
}
