package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/peke/cc-feishu-link/internal/hooks"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("CCFL_HOME", t.TempDir())
	t.Setenv("CCFL_BACKEND", "mock")
	a, err := New(false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Backend != BackendMock {
		t.Fatalf("want mock backend, got %s", a.Backend)
	}
	return a
}

func TestSessionStartAndFirstPrompt(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer

	if err := a.RunSessionStart(&hooks.Input{SessionID: "claude-1", CWD: t.TempDir()}, &buf); err != nil {
		t.Fatal(err)
	}
	id, _ := a.Store.FindByClaudeID("claude-1")
	if id == "" {
		t.Fatal("session not created")
	}
	st, _ := a.Store.Load(id)
	if len(st.Docs) != 6 {
		t.Fatalf("want 6 docs, got %d", len(st.Docs))
	}

	buf.Reset()
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-1",
		Prompt: "Add user authentication to login"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ = a.Store.Load(id)
	if !st.FirstPromptSeen {
		t.Fatal("first prompt not recorded")
	}
	if st.Title == "" || st.Title == "Untitled" {
		t.Fatalf("title not set: %q", st.Title)
	}
	// immediate event should have flushed -> dirty cleared
	if st.Dirty.DirtyEventCount != 0 {
		t.Fatalf("expected flush to clear dirty, got %d", st.Dirty.DirtyEventCount)
	}
}

func TestPreToolUseBlocksDanger(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)

	buf.Reset()
	in := &hooks.Input{SessionID: "c", ToolName: "Bash", ToolInput: []byte(`{"command":"rm -rf /tmp/x"}`)}
	if err := a.RunPreToolUse(in, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"permissionDecision":"deny"`) {
		t.Fatalf("expected deny, got %s", buf.String())
	}
}

func TestPreToolUseAsksForTestEdit(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)

	buf.Reset()
	in := &hooks.Input{SessionID: "c", ToolName: "Edit", ToolInput: []byte(`{"file_path":"tests/test_x.py"}`)}
	if err := a.RunPreToolUse(in, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"permissionDecision":"ask"`) {
		t.Fatalf("expected ask, got %s", buf.String())
	}
}

func TestPostToolUseBuffersNoImmediateSync(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)
	id, _ := a.Store.FindByClaudeID("c")

	// a plain Read should buffer but not sync
	in := &hooks.Input{SessionID: "c", ToolName: "Read", ToolInput: []byte(`{"file_path":"a.go"}`)}
	if err := a.RunPostToolUse(in, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ := a.Store.Load(id)
	if st.Dirty.DirtyEventCount != 1 {
		t.Fatalf("want 1 dirty event, got %d", st.Dirty.DirtyEventCount)
	}
	if !st.LastSyncAt.IsZero() {
		t.Fatal("plain tool should not have synced")
	}
}

func TestPostToolUseValidationTriggersSync(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)
	id, _ := a.Store.FindByClaudeID("c")

	in := &hooks.Input{SessionID: "c", ToolName: "Bash", ToolInput: []byte(`{"command":"go test ./..."}`)}
	if err := a.RunPostToolUse(in, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ := a.Store.Load(id)
	if st.LastSyncAt.IsZero() {
		t.Fatal("validation command should trigger sync")
	}
	if st.Dirty.DirtyEventCount != 0 {
		t.Fatalf("sync should clear dirty, got %d", st.Dirty.DirtyEventCount)
	}
}

func TestStopGeneratesCheckpoint(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)
	if err := a.RunStop(&hooks.Input{SessionID: "c"}, &buf); err != nil {
		t.Fatal(err)
	}
	id, _ := a.Store.FindByClaudeID("c")
	st, _ := a.Store.Load(id)
	if st.LastSyncAt.IsZero() {
		t.Fatal("stop should flush")
	}
}
