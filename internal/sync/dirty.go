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

// MarkContract flags a pending task-contract update (v2).
func MarkContract(st *session.SessionState, now time.Time) {
	st.Dirty.HasContractUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkCockpit flags a pending cockpit update (v2).
func MarkCockpit(st *session.SessionState, now time.Time) {
	st.Dirty.HasCockpitUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkRecap flags a pending recap update (v2).
func MarkRecap(st *session.SessionState, now time.Time) {
	st.Dirty.HasRecapUpdate = true
	st.Dirty.LastDirtyAt = now
}

// MarkMemory flags a pending memory update (v2).
func MarkMemory(st *session.SessionState, now time.Time) {
	st.Dirty.HasMemoryUpdate = true
	st.Dirty.LastDirtyAt = now
}

// Clear resets dirty state after a successful sync.
func Clear(st *session.SessionState, now time.Time) {
	st.Dirty = session.DirtyState{}
	st.LastSyncAt = now
}
