package sync

import (
	"time"

	"github.com/PEKEW/CCF/internal/session"
)

// MarkEvent records that a new buffered event made the session dirty.
func MarkEvent(st *session.SessionState, now time.Time) {
	st.Dirty.HasDirtyEvents = true
	st.Dirty.DirtyEventCount++
	st.Dirty.LastDirtyAt = now
}

// MarkValidation flags a pending validation update.
func MarkValidation(st *session.SessionState, now time.Time) {
	st.Dirty.HasValidationUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkDecision flags a pending decision update.
func MarkDecision(st *session.SessionState, now time.Time) {
	st.Dirty.HasDecisionUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkCompact flags a pending compact update.
func MarkCompact(st *session.SessionState, now time.Time) {
	st.Dirty.HasCompactUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkHandoff flags a pending handoff update.
func MarkHandoff(st *session.SessionState, now time.Time) {
	st.Dirty.HasHandoffUpdate = true
	st.Dirty.LastDirtyAt = now
}

// Clear resets dirty state after a successful sync.
func Clear(st *session.SessionState, now time.Time) {
	st.Dirty = session.DirtyState{}
	st.LastSyncAt = now
}
