package mcp

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PEKEW/CCF/internal/app"
	"github.com/PEKEW/CCF/internal/hooks"
)

func newMockServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	t.Setenv("CCFL_HOME", t.TempDir())
	t.Setenv("CCFL_BACKEND", "mock")
	a, err := app.New(false)
	if err != nil {
		t.Fatal(err)
	}
	// Create a v2 session for tools to act on.
	if err := a.RunSessionStart(&hooks.Input{SessionID: "claude-1", CWD: t.TempDir()}, io.Discard); err != nil {
		t.Fatal(err)
	}
	return NewServer(a), a
}

// mockContains scans all mock markdown files for a substring.
func mockContains(t *testing.T, sub string) bool {
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

func TestServeHandshakeAndList(t *testing.T) {
	s, _ := newMockServer(t)
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}, "\n") + "\n"
	var out strings.Builder
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := nonEmptyLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("want 2 responses (notification suppressed), got %d:\n%s", len(lines), out.String())
	}
	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	json.Unmarshal([]byte(lines[0]), &initResp)
	if initResp.Result.ProtocolVersion != protocolVersion {
		t.Fatalf("bad protocol version: %q", initResp.Result.ProtocolVersion)
	}
	var listResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	json.Unmarshal([]byte(lines[1]), &listResp)
	if len(listResp.Result.Tools) != 8 {
		t.Fatalf("want 8 tools, got %d", len(listResp.Result.Tools))
	}
}

func TestToolSetContract(t *testing.T) {
	s, a := newMockServer(t)
	out, err := s.callTool("feishu_set_contract", json.RawMessage(`{
		"goal":"Ship auth","in_scope":["login"],
		"acceptance_criteria":[{"text":"login works","done":true}],
		"constraints":["no test weakening"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "contract updated") {
		t.Fatalf("unexpected result: %s", out)
	}
	id, _ := a.Store.FindByClaudeID("claude-1")
	st, _ := a.Store.Load(id)
	if st.Contract.Goal != "Ship auth" || len(st.Contract.AcceptanceCriteria) != 1 {
		t.Fatalf("contract not persisted: %+v", st.Contract)
	}
	if !st.Dirty.HasContractUpdate {
		t.Fatal("contract dirty flag not set")
	}
	if !mockContains(t, "Ship auth") {
		t.Fatal("contract goal not written to a mock doc")
	}
}

func TestToolUpdateCockpitAndMemory(t *testing.T) {
	s, a := newMockServer(t)
	if _, err := s.callTool("feishu_update_cockpit", json.RawMessage(`{"summary":"refactoring","health":"yellow","next_step":"add tests"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.callTool("feishu_append_memory", json.RawMessage(`{"text":"keep API stable","kind":"constraint"}`)); err != nil {
		t.Fatal(err)
	}
	id, _ := a.Store.FindByClaudeID("claude-1")
	st, _ := a.Store.Load(id)
	if st.Cockpit.Summary != "refactoring" || st.Cockpit.Health != "yellow" {
		t.Fatalf("cockpit not persisted: %+v", st.Cockpit)
	}
	if len(st.Memory) != 1 || st.Memory[0].Text != "keep API stable" {
		t.Fatalf("memory not persisted: %+v", st.Memory)
	}
	if !mockContains(t, "refactoring") || !mockContains(t, "keep API stable") {
		t.Fatal("cockpit/memory not written to mock docs")
	}
}

func TestUnknownToolIsProtocolError(t *testing.T) {
	s, _ := newMockServer(t)
	in := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"nope","arguments":{}}}` + "\n"
	var out strings.Builder
	_ = s.Serve(strings.NewReader(in), &out)
	if !strings.Contains(out.String(), `"error"`) || !strings.Contains(out.String(), "unknown tool") {
		t.Fatalf("expected protocol error for unknown tool: %s", out.String())
	}
}

func TestLegacySessionRefused(t *testing.T) {
	s, a := newMockServer(t)
	// Flip the session to legacy and save.
	id, _ := a.Store.FindByClaudeID("claude-1")
	st, _ := a.Store.Load(id)
	st.DocLayout = ""
	_ = a.Store.Save(st)

	_, err := s.callTool("feishu_update_cockpit", json.RawMessage(`{"summary":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "legacy doc layout") {
		t.Fatalf("expected legacy refusal, got %v", err)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
