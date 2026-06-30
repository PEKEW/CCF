package summary

import (
	"context"
	"os"
	"testing"
)

func TestParseSections(t *testing.T) {
	out := `junk before
===COCKPIT===
现在正在做登录功能,已完成一半,没有卡住。
===RECAP===
## 已完成
- 登录页

## 待办
- 登出
===MEMORY===
- important: 登录是核心功能
- gotcha: 注意 token 过期
随便一行没冒号
`
	r := parse(out)
	if r.Cockpit != "现在正在做登录功能,已完成一半,没有卡住。" {
		t.Fatalf("cockpit: %q", r.Cockpit)
	}
	if got := r.Recap; got == "" || !contains(got, "已完成") || !contains(got, "登出") {
		t.Fatalf("recap: %q", got)
	}
	if len(r.Memory) != 3 {
		t.Fatalf("want 3 memory facts, got %d: %+v", len(r.Memory), r.Memory)
	}
	if r.Memory[0].Kind != "important" || r.Memory[1].Kind != "gotcha" {
		t.Fatalf("memory kinds: %+v", r.Memory)
	}
	if r.Memory[2].Kind != "fact" { // line without a known kind prefix
		t.Fatalf("want fallback fact kind, got %q", r.Memory[2].Kind)
	}
}

func TestParseNoMarkersFallsBackToRecap(t *testing.T) {
	r := parse("just some prose, no markers at all")
	if r.Recap == "" {
		t.Fatal("expected unmarked output to land in recap")
	}
	if r.Cockpit != "" {
		t.Fatalf("cockpit should be empty, got %q", r.Cockpit)
	}
}

func TestGenerateRefusesRecursion(t *testing.T) {
	t.Setenv(EnvGuard, "1")
	_, err := Generate(context.Background(), Options{TranscriptPath: "/tmp/whatever.jsonl"})
	if err == nil {
		t.Fatal("Generate must refuse to run when the recursion guard is set")
	}
}

func TestSplitFact(t *testing.T) {
	cases := map[string][2]string{
		"important: 核心":  {"important", "核心"},
		"gotcha:小心":      {"gotcha", "小心"},
		"no kind here":   {"fact", "no kind here"},
		"unknown: thing": {"fact", "unknown: thing"},
	}
	for in, want := range cases {
		k, txt := splitFact(in)
		if k != want[0] || txt != want[1] {
			t.Fatalf("%q -> (%q,%q), want (%q,%q)", in, k, txt, want[0], want[1])
		}
	}
}

func TestAvailableOverride(t *testing.T) {
	t.Setenv("CCFL_CLAUDE_BIN", "/nonexistent/definitely-not-here")
	if Available() {
		t.Fatal("Available should be false for a missing binary")
	}
	// real `claude` may or may not exist in CI; just ensure it doesn't panic.
	os.Unsetenv("CCFL_CLAUDE_BIN")
	_ = Available()
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
