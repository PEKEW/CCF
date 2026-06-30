package sync

import (
	"strings"
	"testing"
	"time"

	"github.com/PEKEW/CCF/internal/session"
)

func v2State() *session.SessionState {
	return &session.SessionState{
		SessionID: "ls-1", Title: "Demo", Status: "active", Phase: "working",
		DocLayout: session.LayoutV2,
		Contract: session.Contract{
			Goal: "Add auth", InScope: []string{"login"}, OutScope: []string{"sso"},
			AcceptanceCriteria: []session.Criterion{{Text: "login works", Done: true}, {Text: "logout works"}},
			Constraints:        []string{"do not weaken tests"},
		},
		Cockpit: session.Cockpit{Summary: "wiring middleware", NextStep: "write tests", Health: "yellow"},
	}
}

// noEventLog asserts a rendered doc carries no agent event-log markers.
func noEventLog(t *testing.T, s string) {
	t.Helper()
	for _, bad := range []string{"Checkpoint", "[tool]", "[validation]", "Recent Changes", "Files Touched"} {
		if strings.Contains(s, bad) {
			t.Fatalf("event-log marker %q leaked into doc:\n%s", bad, s)
		}
	}
}

func TestRenderCockpit(t *testing.T) {
	st := v2State()
	out := RenderCockpit(st)
	for _, want := range []string{"# Cockpit", "Add auth", "wiring middleware", "write tests", "1/2 criteria", "🟡", "## Metadata"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cockpit missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "## Blocker\n\nnone") {
		t.Fatalf("expected blocker 'none':\n%s", out)
	}
	noEventLog(t, out)
}

func TestRenderContractCriteria(t *testing.T) {
	out := RenderContract(v2State())
	if !strings.Contains(out, "- [x] login works") || !strings.Contains(out, "- [ ] logout works") {
		t.Fatalf("criteria checkboxes wrong:\n%s", out)
	}
	if !strings.Contains(out, "do not weaken tests") {
		t.Fatalf("constraint missing:\n%s", out)
	}
	noEventLog(t, out)
}

func TestRenderRecap(t *testing.T) {
	st := v2State()
	st.RecapNarrative = "Did the thing.\nThen the other thing."
	out := RenderRecap(st)
	if !strings.Contains(out, "Did the thing.") {
		t.Fatalf("recap missing narrative:\n%s", out)
	}
	noEventLog(t, out)
}

func TestRenderMemoryGroupedByKind(t *testing.T) {
	st := v2State()
	st.Memory = []session.MemoryItem{
		{Kind: "constraint", Text: "keep API stable"},
		{Kind: "gotcha", Text: "watch the cache TTL"},
		{Kind: "decision", Text: "use JWT"},
	}
	out := RenderMemory(st)
	for _, want := range []string{"Constraints", "keep API stable", "Gotchas", "watch the cache TTL", "Decisions", "use JWT"} {
		if !strings.Contains(out, want) {
			t.Fatalf("memory missing %q:\n%s", want, out)
		}
	}
	// Gotchas section must come before Constraints (fixed order).
	if strings.Index(out, "Gotchas") > strings.Index(out, "Constraints") {
		t.Fatalf("memory kind order wrong:\n%s", out)
	}
}

func TestRenderHandoffV2(t *testing.T) {
	st := v2State()
	st.Handoff = session.Handoff{
		Done: []string{"wired middleware"}, Remains: []string{"tests"},
		FilesToRead: []string{"auth.go"},
	}
	out := RenderHandoffV2(st)
	for _, want := range []string{"wired middleware", "## What Remains", "tests", "auth.go", "Current State"} {
		if !strings.Contains(out, want) {
			t.Fatalf("handoff missing %q:\n%s", want, out)
		}
	}
	noEventLog(t, out)
}

func TestRenderValidationAndDecisionEntries(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	v := RenderValidationEntry(now, "go test ./... -> PASS")
	if !strings.Contains(v, "### Validation") || !strings.Contains(v, "PASS") {
		t.Fatalf("validation entry wrong: %s", v)
	}
	d := RenderDecisionEntry(now, "BLOCK", "Bash on \"rm -rf\"", "dangerous_delete", "rm blocked")
	for _, want := range []string{"### Decision", "BLOCK", "dangerous_delete", "rm blocked"} {
		if !strings.Contains(d, want) {
			t.Fatalf("decision entry missing %q: %s", want, d)
		}
	}
}
