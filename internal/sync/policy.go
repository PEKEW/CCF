package sync

import (
	"fmt"
	"time"

	"github.com/PEKEW/CCF/internal/session"
)

// Decision is the outcome of evaluating sync policy.
type Decision struct {
	Sync   bool
	Reason string
}

// EvaluateNow evaluates policy at the current time with no trigger event.
func EvaluateNow(st *session.SessionState) Decision {
	return Evaluate(st, time.Now(), "")
}

// Evaluate decides whether a sync should happen now.
//
// triggerEvent is the kind of the event that just occurred (may be ""). It is
// matched against the policy's ImmediateEvents list. Deterministic: same inputs
// always produce the same decision.
func Evaluate(st *session.SessionState, now time.Time, triggerEvent string) Decision {
	p := st.SyncPolicy
	if p.Mode == "off" || p.Mode == "never" {
		return Decision{false, "sync disabled by policy"}
	}

	if triggerEvent != "" {
		for _, ie := range p.ImmediateEvents {
			if ie == triggerEvent {
				return Decision{true, "immediate event: " + triggerEvent}
			}
		}
	}

	d := st.Dirty
	if !d.HasDirtyEvents && !d.HasValidationUpdate && !d.HasDecisionUpdate &&
		!d.HasCompactUpdate && !d.HasHandoffUpdate {
		return Decision{false, "nothing dirty"}
	}

	if p.MinDirtyEvents > 0 && d.DirtyEventCount >= p.MinDirtyEvents {
		return Decision{true, fmt.Sprintf("dirty events %d >= %d", d.DirtyEventCount, p.MinDirtyEvents)}
	}

	if p.MaxUnsyncedMinutes > 0 {
		base := st.LastSyncAt
		if base.IsZero() {
			base = st.CreatedAt
		}
		if !base.IsZero() {
			age := now.Sub(base)
			if age >= time.Duration(p.MaxUnsyncedMinutes)*time.Minute {
				return Decision{true, fmt.Sprintf("unsynced for %s", age.Round(time.Minute))}
			}
		}
	}

	return Decision{false, "below sync thresholds"}
}
