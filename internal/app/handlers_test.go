package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PEKEW/CCF/internal/hooks"
	"github.com/PEKEW/CCF/internal/session"
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
	if !st.IsV2() {
		t.Fatalf("new session should use v2 layout, got %q", st.DocLayout)
	}
	// Folder + docs are created lazily on the first prompt, not at session-start.
	if len(st.Docs) != 0 {
		t.Fatalf("want 0 docs before first prompt, got %d", len(st.Docs))
	}

	buf.Reset()
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-1",
		Prompt: "Add user authentication to login"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ = a.Store.Load(id)
	if len(st.Docs) != 6 {
		t.Fatalf("want 6 docs after first prompt, got %d", len(st.Docs))
	}
	if st.FeishuFolderToken == "" {
		t.Fatal("folder should be created on first prompt")
	}
	if !st.FirstPromptSeen {
		t.Fatal("first prompt not recorded")
	}
	if st.Title == "" || st.Title == "Untitled" {
		t.Fatalf("title not set: %q", st.Title)
	}
	if st.Contract.Goal != st.Title {
		t.Fatalf("first prompt should seed contract goal, got %q", st.Contract.Goal)
	}
	// immediate event should have flushed -> dirty cleared
	if st.Dirty.DirtyEventCount != 0 {
		t.Fatalf("expected flush to clear dirty, got %d", st.Dirty.DirtyEventCount)
	}
}

func TestInjectedFirstPromptSkipped(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "claude-1", CWD: t.TempDir()}, &buf)
	id, _ := a.Store.FindByClaudeID("claude-1")

	// harness injection arrives first — must NOT become the title
	buf.Reset()
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-1",
		Prompt: "<task-notification>build done</task-notification>"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ := a.Store.Load(id)
	if st.FirstPromptSeen {
		t.Fatal("injected prompt should not count as first prompt")
	}
	if st.Title != "Untitled" {
		t.Fatalf("title should stay Untitled, got %q", st.Title)
	}

	// real prompt next — this one titles the session
	buf.Reset()
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-1",
		Prompt: "Add user authentication to login"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ = a.Store.Load(id)
	if !st.FirstPromptSeen {
		t.Fatal("real prompt should be recorded as first")
	}
	if !strings.Contains(strings.ToLower(st.Title), "authentication") {
		t.Fatalf("title should come from real prompt, got %q", st.Title)
	}
}

func TestHookNoOpWhenSessionMissing(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	// no session-start for this id; must not error or bind to a stale session
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: "ghost", Prompt: "hello"}, &buf); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
	if !strings.Contains(buf.String(), `"continue":true`) && buf.Len() == 0 {
		// Allow() output; just ensure no session was created
	}
	if id, _ := a.Store.FindByClaudeID("ghost"); id != "" {
		t.Fatal("no-op hook must not create a session")
	}
}

func TestStaleSessionNotHijacked(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	// real session for claude-A
	_ = a.RunSessionStart(&hooks.Input{SessionID: "claude-A", CWD: t.TempDir()}, &buf)
	idA, _ := a.Store.FindByClaudeID("claude-A")

	// claude-B has no session (its session-start "failed"); its prompt must NOT
	// mutate claude-A's session via a latest() fallback.
	buf.Reset()
	_ = a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-B", Prompt: "different task"}, &buf)
	stA, _ := a.Store.Load(idA)
	if stA.FirstPromptSeen {
		t.Fatal("claude-A session was hijacked by claude-B prompt")
	}
}

func TestResumeReusesSessionAndReactivates(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "claude-1", CWD: t.TempDir()}, &buf)
	id, _ := a.Store.FindByClaudeID("claude-1")
	_ = a.RunUserPromptSubmit(&hooks.Input{SessionID: "claude-1", Prompt: "do a thing"}, &buf)

	// end the session
	_ = a.RunSessionEnd(&hooks.Input{SessionID: "claude-1", Reason: "exit"}, &buf)
	st, _ := a.Store.Load(id)
	if st.Status != "ended" {
		t.Fatalf("want ended, got %q", st.Status)
	}

	// resume: same id, no new session/folder, status back to active
	folderBefore := st.FeishuFolderURL
	buf.Reset()
	if err := a.RunSessionStart(&hooks.Input{SessionID: "claude-1", CWD: t.TempDir()}, &buf); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Store.FindByClaudeID("claude-1")
	if got != id {
		t.Fatalf("resume created a new session: %s != %s", got, id)
	}
	st, _ = a.Store.Load(id)
	if st.Status != "active" {
		t.Fatalf("resume should reactivate, got %q", st.Status)
	}
	if st.FeishuFolderURL != folderBefore {
		t.Fatalf("resume changed folder: %q != %q", st.FeishuFolderURL, folderBefore)
	}
	if !strings.Contains(buf.String(), "resumed") {
		t.Fatalf("expected resume context, got %s", buf.String())
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
	if len(st.Validations) != 1 {
		t.Fatalf("validation should be recorded in state, got %d", len(st.Validations))
	}
}

func TestStopWritesHandoff(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)
	_ = a.RunUserPromptSubmit(&hooks.Input{SessionID: "c", Prompt: "do the thing"}, &buf)
	id, _ := a.Store.FindByClaudeID("c")
	st, _ := a.Store.Load(id)
	st.Handoff.Done = []string{"did a thing"}
	_ = a.Store.Save(st)

	if err := a.RunStop(&hooks.Input{SessionID: "c"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ = a.Store.Load(id)
	if st.LastSyncAt.IsZero() {
		t.Fatal("stop should flush")
	}
	// v2 stop must write the handoff doc and never a checkpoint/event dump.
	if !mockFileContains(t, "did a thing") {
		t.Fatal("handoff content not written")
	}
	if mockFileContains(t, "Checkpoint") {
		t.Fatal("v2 stop must not write an event-log checkpoint")
	}
}

func TestPreCompactDistillsMemory(t *testing.T) {
	a := newTestApp(t)
	var buf bytes.Buffer
	_ = a.RunSessionStart(&hooks.Input{SessionID: "c", CWD: t.TempDir()}, &buf)
	_ = a.RunUserPromptSubmit(&hooks.Input{SessionID: "c", Prompt: "do the thing"}, &buf)
	id, _ := a.Store.FindByClaudeID("c")
	st, _ := a.Store.Load(id)
	st.Contract.Constraints = []string{"do not weaken tests"}
	st.AppendDecision(session.LogEntry{Kind: "decision", Text: "use JWT"})
	_ = a.Store.Save(st)

	if err := a.RunPreCompact(&hooks.Input{SessionID: "c", Trigger: "auto"}, &buf); err != nil {
		t.Fatal(err)
	}
	st, _ = a.Store.Load(id)
	if len(st.Memory) == 0 {
		t.Fatal("pre-compact should distill memory")
	}
	if !mockFileContains(t, "do not weaken tests") {
		t.Fatal("distilled memory not written to doc")
	}
}

// mockFileContains scans the mock backend output for a substring.
func mockFileContains(t *testing.T, sub string) bool {
	t.Helper()
	root := filepath.Join(os.Getenv("CCFL_HOME"), ".mock-feishu")
	found := false
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), sub) {
			found = true
		}
		return nil
	})
	return found
}
