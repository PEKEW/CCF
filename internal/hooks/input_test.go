package hooks

import (
	"strings"
	"testing"
)

func TestParseInputPreToolUse(t *testing.T) {
	body := `{
		"session_id": "abc",
		"cwd": "/repo",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf x"}
	}`
	in, err := ParseInput(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if in.SessionID != "abc" || in.ToolName != "Bash" {
		t.Fatalf("bad parse: %+v", in)
	}
	f := in.ToolFields()
	if f.Command != "rm -rf x" {
		t.Fatalf("bad command: %q", f.Command)
	}
}

func TestParseInputEmpty(t *testing.T) {
	in, err := ParseInput(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if in.SessionID != "" {
		t.Fatalf("expected zero input")
	}
}

func TestToolFieldsPathAlias(t *testing.T) {
	in := &Input{ToolInput: []byte(`{"path": "/a/b.txt"}`)}
	if in.ToolFields().FilePath != "/a/b.txt" {
		t.Fatal("path alias not applied")
	}
}

func TestOutputDeny(t *testing.T) {
	var sb strings.Builder
	if err := Deny("nope").Write(&sb); err != nil {
		t.Fatal(err)
	}
	s := sb.String()
	if !strings.Contains(s, `"permissionDecision":"deny"`) || !strings.Contains(s, "nope") {
		t.Fatalf("bad deny output: %s", s)
	}
}
