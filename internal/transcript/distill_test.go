package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDistill(t *testing.T) {
	lines := []string{
		`{"type":"ai-title","aiTitle":"Fix auth"}`,
		`{"type":"user","message":{"role":"user","content":"Add login to the app"}}`,
		`{"type":"user","message":{"role":"user","content":"<task-notification>ignore me</task-notification>"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll edit the handler."},{"type":"tool_use","name":"Edit","input":{"file_path":"auth.go"}}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","message":{"role":"user","content":"now add logout"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done. Logout added."}]}}`,
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "t.jsonl")
	if err := os.WriteFile(p, []byte(join(lines)), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := Distill(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Fix auth" {
		t.Fatalf("title: %q", d.Title)
	}
	if len(d.Prompts) != 2 { // injected one filtered
		t.Fatalf("want 2 prompts, got %d: %v", len(d.Prompts), d.Prompts)
	}
	if d.LastUserPrompt != "now add logout" {
		t.Fatalf("last prompt: %q", d.LastUserPrompt)
	}
	if d.LastAssistantText != "Done. Logout added." {
		t.Fatalf("last assistant: %q", d.LastAssistantText)
	}
	if len(d.EditedFiles) != 1 || d.EditedFiles[0] != "auth.go" {
		t.Fatalf("edited files: %v", d.EditedFiles)
	}
	if d.Validations != 1 {
		t.Fatalf("validations: %d", d.Validations)
	}
}

func TestDistillMissingFile(t *testing.T) {
	d, err := Distill("/no/such/file.jsonl")
	if err != nil || d == nil || len(d.Prompts) != 0 {
		t.Fatalf("missing file should yield empty digest, no error: %v %v", d, err)
	}
}

func join(xs []string) string {
	out := ""
	for _, x := range xs {
		out += x + "\n"
	}
	return out
}
