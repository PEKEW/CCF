package sync

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Sync priority values.
const (
	PriorityLow       = "low"
	PriorityNormal    = "normal"
	PriorityImportant = "important"
	PriorityImmediate = "immediate"
	PriorityNever     = "never"
)

// Event is one buffered hook/tool event.
type Event struct {
	ID           string          `json:"id"`
	Time         time.Time       `json:"time"`
	Kind         string          `json:"kind"`
	HookEvent    string          `json:"hook_event,omitempty"`
	ToolName     string          `json:"tool_name,omitempty"`
	Summary      string          `json:"summary,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Sensitive    bool            `json:"sensitive"`
	SyncPriority string          `json:"sync_priority"`
}

// NewEventID returns a random event id.
func NewEventID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "ev-" + hex.EncodeToString(b[:])
}

// Buffer is an append-only event log backed by events.jsonl.
type Buffer struct {
	Path string
}

// NewBuffer returns a Buffer for events.jsonl inside sessionDir.
func NewBuffer(sessionDir string) *Buffer {
	return &Buffer{Path: filepath.Join(sessionDir, "events.jsonl")}
}

// Append writes one event as a JSON line.
func (b *Buffer) Append(e Event) error {
	if e.ID == "" {
		e.ID = NewEventID()
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.SyncPriority == "" {
		e.SyncPriority = PriorityNormal
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(b.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// All reads every event in order. Missing file returns an empty slice.
func (b *Buffer) All() ([]Event, error) {
	f, err := os.Open(b.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(b, &e); err != nil {
			continue // skip corrupt line
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// Since returns events with Time strictly after t.
func (b *Buffer) Since(t time.Time) ([]Event, error) {
	all, err := b.All()
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, e := range all {
		if e.Time.After(t) {
			out = append(out, e)
		}
	}
	return out, nil
}
