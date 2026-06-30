package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/PEKEW/CCF/internal/hooks"
	"github.com/PEKEW/CCF/internal/templates"
)

// startSession runs session-start + a real first prompt so the folder and the
// full doc set (including 06_MEMO) exist.
func startSession(t *testing.T, a *App, claudeID, prompt string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := a.RunSessionStart(&hooks.Input{SessionID: claudeID, CWD: t.TempDir()}, &buf); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := a.RunUserPromptSubmit(&hooks.Input{SessionID: claudeID, Prompt: prompt}, &buf); err != nil {
		t.Fatal(err)
	}
	id, _ := a.Store.FindByClaudeID(claudeID)
	if id == "" {
		t.Fatal("session not created")
	}
	return id
}

func TestMemoReadBackOnResume(t *testing.T) {
	a := newTestApp(t)
	id := startSession(t, a, "claude-memo", "build a thing")
	st, _ := a.Store.Load(id)

	// Simulate the human editing the memo's HUMAN section in Feishu.
	humanEdited := "# 备忘录 Memo\n\n" + templates.MemoHumanStart +
		"\n## 人工备注\n\n务必使用 PostgreSQL\n" + templates.MemoHumanEnd +
		"\n\n" + templates.MemoClaudeStart + "\n(none yet)\n" + templates.MemoClaudeEnd + "\n"
	if err := a.PushDoc(st, string(templates.KeyMemo), humanEdited); err != nil {
		t.Fatal(err)
	}

	// Resume: session-start with the same id must inject the human note.
	var buf bytes.Buffer
	if err := a.RunSessionStart(&hooks.Input{SessionID: "claude-memo"}, &buf); err != nil {
		t.Fatal(err)
	}
	var out hooks.Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v (%s)", err, buf.String())
	}
	if out.HookSpecific == nil || !strings.Contains(out.HookSpecific.AdditionalContext, "PostgreSQL") {
		t.Fatalf("resume did not inject human memo, got: %+v", out.HookSpecific)
	}
}

func TestUpdateMemoPreservesHumanEdit(t *testing.T) {
	a := newTestApp(t)
	id := startSession(t, a, "claude-memo2", "build a thing")
	st, _ := a.Store.Load(id)

	// Human writes a note into the memo.
	humanEdited := "# 备忘录 Memo\n\n" + templates.MemoHumanStart +
		"\n## 人工备注\n\n不要动 CI 配置\n" + templates.MemoHumanEnd +
		"\n\n" + templates.MemoClaudeStart + "\n(none yet)\n" + templates.MemoClaudeEnd + "\n"
	if err := a.PushDoc(st, string(templates.KeyMemo), humanEdited); err != nil {
		t.Fatal(err)
	}

	// Claude updates its own notes; the human section must survive.
	st.MemoNotes = []string{"已经加了登录"}
	if err := a.updateMemo(st); err != nil {
		t.Fatal(err)
	}
	got, err := a.readDocText(st, string(templates.KeyMemo))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "不要动 CI 配置") {
		t.Fatalf("human edit lost after claude memo update:\n%s", got)
	}
	if !strings.Contains(got, "已经加了登录") {
		t.Fatalf("claude note not written:\n%s", got)
	}
}
