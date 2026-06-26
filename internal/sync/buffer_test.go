package sync

import (
	"testing"
	"time"
)

func TestBufferAppendAndAll(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(dir)
	for i := 0; i < 3; i++ {
		if err := b.Append(Event{Kind: "tool", Summary: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := b.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3, got %d", len(all))
	}
	for _, e := range all {
		if e.ID == "" || e.Time.IsZero() || e.SyncPriority == "" {
			t.Fatalf("event not defaulted: %+v", e)
		}
	}
}

func TestBufferSince(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(dir)
	old := time.Now().Add(-time.Hour)
	_ = b.Append(Event{Kind: "old", Time: old})
	cut := time.Now()
	time.Sleep(2 * time.Millisecond)
	_ = b.Append(Event{Kind: "new"})

	got, err := b.Since(cut)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != "new" {
		t.Fatalf("Since wrong: %+v", got)
	}
}

func TestBufferEmpty(t *testing.T) {
	b := NewBuffer(t.TempDir())
	all, err := b.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("want empty, got %d", len(all))
	}
}
