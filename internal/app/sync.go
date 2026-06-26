package app

import (
	"time"

	"github.com/peke/cc-feishu-link/internal/session"
	syncpkg "github.com/peke/cc-feishu-link/internal/sync"
	"github.com/peke/cc-feishu-link/internal/templates"
)

// flush renders a checkpoint from buffered events and writes it plus the index
// and active-context docs, then clears dirty state. reason is logged.
func (a *App) flush(st *session.SessionState, reason string) error {
	now := time.Now()
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	events, err := buf.Since(st.LastSyncAt)
	if err != nil {
		return err
	}

	a.notef("sync: %s (%d events since last sync)", reason, len(events))

	cp := syncpkg.RenderCheckpoint(st, events, now)
	if err := a.appendDoc(st, string(templates.KeyCheckpoints), cp); err != nil {
		return err
	}
	if err := a.updateDoc(st, string(templates.KeyIndex), syncpkg.RenderIndex(st)); err != nil {
		return err
	}
	if err := a.updateDoc(st, string(templates.KeyActiveContext), syncpkg.RenderActiveContext(st, events)); err != nil {
		return err
	}

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
