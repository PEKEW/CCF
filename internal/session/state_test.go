package session

import (
	"encoding/json"
	"testing"
	"time"
)

// TestLegacyJSONBackCompat: a pre-v2 session.json (no new fields) unmarshals
// cleanly with zero-valued new structs and an empty (legacy) DocLayout.
func TestLegacyJSONBackCompat(t *testing.T) {
	legacy := `{
		"session_id":"ls-old","status":"active","phase":"working","title":"old",
		"docs":{"00_SESSION_INDEX":{"name":"00","token":"t"}},
		"dirty":{"has_dirty_events":true,"dirty_event_count":3}
	}`
	var st SessionState
	if err := json.Unmarshal([]byte(legacy), &st); err != nil {
		t.Fatal(err)
	}
	if st.IsV2() {
		t.Fatal("legacy session must not be v2")
	}
	if st.DocLayout != "" {
		t.Fatalf("expected empty layout, got %q", st.DocLayout)
	}
	if st.Contract.Goal != "" || len(st.Memory) != 0 || st.RecapNarrative != "" {
		t.Fatal("new fields should be zero on legacy state")
	}
	if !st.Dirty.Pending() {
		t.Fatal("legacy dirty events should be pending")
	}
}

// TestV2RoundTrip: marshal a v2 state and re-unmarshal to an equal value.
func TestV2RoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	in := SessionState{
		SessionID: "ls-1", DocLayout: LayoutV2, Status: "active",
		Contract: Contract{Goal: "g", AcceptanceCriteria: []Criterion{{Text: "a", Done: true}}, UpdatedAt: now},
		Cockpit:  Cockpit{Summary: "s", Health: "green"},
		Memory:   []MemoryItem{{Time: now, Kind: "fact", Text: "x"}},
		Handoff:  Handoff{Done: []string{"d"}, UpdatedAt: now},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out SessionState
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !out.IsV2() || out.Contract.Goal != "g" || out.Cockpit.Health != "green" ||
		len(out.Memory) != 1 || out.Memory[0].Text != "x" || len(out.Handoff.Done) != 1 {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
	if !out.Contract.AcceptanceCriteria[0].Done {
		t.Fatal("criterion done lost in round-trip")
	}
}

// TestAppendCapped bounds the in-state decision/validation tails.
func TestAppendCapped(t *testing.T) {
	var st SessionState
	for i := 0; i < logCap+10; i++ {
		st.AppendDecision(LogEntry{Kind: "decision", Text: "d"})
	}
	if len(st.Decisions) != logCap {
		t.Fatalf("want %d capped decisions, got %d", logCap, len(st.Decisions))
	}
}
