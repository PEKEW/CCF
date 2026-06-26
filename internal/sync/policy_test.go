package sync

import (
	"testing"
	"time"

	"github.com/PEKEW/CCF/internal/session"
)

func baseState() *session.SessionState {
	st := &session.SessionState{
		CreatedAt:  time.Now(),
		SyncPolicy: session.DefaultSyncPolicy(),
	}
	return st
}

func TestImmediateEventTriggers(t *testing.T) {
	st := baseState()
	d := Evaluate(st, time.Now(), "validation_completed")
	if !d.Sync {
		t.Fatalf("expected sync for immediate event, got %q", d.Reason)
	}
}

func TestNormalEventNoSync(t *testing.T) {
	st := baseState()
	st.Dirty.HasDirtyEvents = true
	st.Dirty.DirtyEventCount = 1
	d := Evaluate(st, time.Now(), "")
	if d.Sync {
		t.Fatalf("did not expect sync: %q", d.Reason)
	}
}

func TestDirtyCountTriggers(t *testing.T) {
	st := baseState()
	st.Dirty.HasDirtyEvents = true
	st.Dirty.DirtyEventCount = 5 // == MinDirtyEvents default
	d := Evaluate(st, time.Now(), "")
	if !d.Sync {
		t.Fatalf("expected sync at threshold, got %q", d.Reason)
	}
}

func TestMaxUnsyncedTriggers(t *testing.T) {
	st := baseState()
	st.Dirty.HasDirtyEvents = true
	st.Dirty.DirtyEventCount = 1
	st.CreatedAt = time.Now().Add(-31 * time.Minute)
	d := Evaluate(st, time.Now(), "")
	if !d.Sync {
		t.Fatalf("expected sync after max unsynced, got %q", d.Reason)
	}
}

func TestNothingDirty(t *testing.T) {
	st := baseState()
	d := Evaluate(st, time.Now(), "")
	if d.Sync {
		t.Fatalf("expected no sync when clean: %q", d.Reason)
	}
}
