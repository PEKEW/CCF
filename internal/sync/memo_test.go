package sync

import (
	"strings"
	"testing"

	"github.com/PEKEW/CCF/internal/templates"
)

func TestMemoHumanTextFromTemplate(t *testing.T) {
	// Untouched template => no real human note.
	if got := MemoHumanText(templates.MemoTmpl); got != "" {
		t.Fatalf("template should yield empty human text, got %q", got)
	}
}

func TestMemoHumanTextExtracts(t *testing.T) {
	raw := "# 备忘录 Memo\n\n" + templates.MemoHumanStart +
		"\n## 人工备注\n\n用 PostgreSQL 不要用 MySQL\n记得跑 lint\n" + templates.MemoHumanEnd +
		"\n\n" + templates.MemoClaudeStart + "\n旧的 claude 备注\n" + templates.MemoClaudeEnd + "\n"
	got := MemoHumanText(raw)
	if !strings.Contains(got, "PostgreSQL") || !strings.Contains(got, "lint") {
		t.Fatalf("human text missing content: %q", got)
	}
	if strings.Contains(got, "claude 备注") {
		t.Fatalf("human text leaked claude section: %q", got)
	}
}

func TestRenderMemoPreservesHuman(t *testing.T) {
	human := "## 人工备注\n\n保持 API 兼容"
	out := RenderMemo(human, []string{"已完成登录", "注意 token 过期"})
	if !strings.Contains(out, "保持 API 兼容") {
		t.Fatalf("memo dropped human section:\n%s", out)
	}
	if !strings.Contains(out, "已完成登录") || !strings.Contains(out, "token 过期") {
		t.Fatalf("memo dropped claude notes:\n%s", out)
	}
	// Markers must be present so the next round-trip can re-extract sections.
	for _, m := range []string{templates.MemoHumanStart, templates.MemoHumanEnd, templates.MemoClaudeStart, templates.MemoClaudeEnd} {
		if !strings.Contains(out, m) {
			t.Fatalf("memo missing marker %q:\n%s", m, out)
		}
	}
	// Round-trip: extracting the human section back returns it intact.
	if got := ExtractSection(out, templates.MemoHumanStart, templates.MemoHumanEnd); !strings.Contains(got, "保持 API 兼容") {
		t.Fatalf("round-trip lost human section: %q", got)
	}
}

func TestRenderMemoEmptyClaude(t *testing.T) {
	out := RenderMemo("", nil)
	if !strings.Contains(out, "none yet") {
		t.Fatalf("empty claude notes should show placeholder:\n%s", out)
	}
}
