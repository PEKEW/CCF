// Package summary turns a session transcript into human-readable prose by
// shelling out to a headless `claude -p` call. It is the "hybrid" engine: a
// deterministic digest (from internal/transcript) is fed to Claude, which
// rewrites it as plain language a non-coder can read.
//
// Recursion: the headless call spawns a new Claude session, which fires ccfl
// hooks again. To avoid an infinite loop the child is launched with
// CCFL_INSIDE_SUMMARY=1; ccfl hooks no-op when that env var is set (see
// cmd/ccfl). This package never runs when that guard is already active.
package summary

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/PEKEW/CCF/internal/transcript"
)

// EnvGuard is set on the headless child so re-entrant ccfl hooks no-op.
const EnvGuard = "CCFL_INSIDE_SUMMARY"

// MemoryFact is one durable fact extracted for 05_MEMORY.
type MemoryFact struct {
	Kind string // important|constraint|gotcha|decision|resource|fact
	Text string
}

// Result is the prose the engine produced.
type Result struct {
	Cockpit string       // 2-3 plain sentences: what's happening now
	Recap   string       // markdown narrative for 02_RECAP
	Memory  []MemoryFact // durable facts for 05_MEMORY
}

// Options configures a Generate call.
type Options struct {
	TranscriptPath string
	Goal           string
	PrevRecap      string
	Lang           string // "zh" (default) | "en"
}

const maxCtx = 6000 // cap chars of distilled context fed to claude

// claudeBin returns the claude executable to invoke (override via CCFL_CLAUDE_BIN).
func claudeBin() string {
	if b := os.Getenv("CCFL_CLAUDE_BIN"); b != "" {
		return b
	}
	return "claude"
}

// Available reports whether a claude binary can be found on PATH.
func Available() bool {
	_, err := exec.LookPath(claudeBin())
	return err == nil
}

// Generate distills the transcript and asks claude to rewrite it as prose.
// Returns an error if the transcript is empty, claude is missing, or the call
// fails — callers fall back to the deterministic skeleton on error.
func Generate(ctx context.Context, opt Options) (*Result, error) {
	if os.Getenv(EnvGuard) != "" {
		return nil, fmt.Errorf("summary: refusing to recurse (%s set)", EnvGuard)
	}
	if !Available() {
		return nil, fmt.Errorf("summary: claude binary not found")
	}
	dg, _ := transcript.Distill(opt.TranscriptPath)
	if len(dg.Prompts) == 0 && len(dg.EditedFiles) == 0 && dg.LastAssistantText == "" {
		return nil, fmt.Errorf("summary: empty transcript")
	}

	prompt := buildPrompt(opt, dg)
	cmd := exec.CommandContext(ctx, claudeBin(), "-p", prompt)
	cmd.Env = append(os.Environ(), EnvGuard+"=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("summary: claude -p failed: %w", err)
	}
	return parse(string(out)), nil
}

func buildPrompt(opt Options, dg *transcript.Digest) string {
	lang := opt.Lang
	if lang == "" {
		lang = "zh"
	}
	var c strings.Builder
	if opt.Goal != "" {
		fmt.Fprintf(&c, "任务目标: %s\n", opt.Goal)
	} else if dg.Title != "" {
		fmt.Fprintf(&c, "任务目标: %s\n", dg.Title)
	}
	ps := dg.Prompts
	if len(ps) > 10 {
		ps = ps[len(ps)-10:]
	}
	if len(ps) > 0 {
		c.WriteString("\n用户的请求(按时间):\n")
		for _, p := range ps {
			fmt.Fprintf(&c, "- %s\n", oneLine(trunc(p, 200)))
		}
	}
	if len(dg.EditedFiles) > 0 {
		files := dg.EditedFiles
		if len(files) > 25 {
			files = files[len(files)-25:]
		}
		fmt.Fprintf(&c, "\n改动的文件(%d个): %s\n", len(dg.EditedFiles), strings.Join(files, ", "))
	}
	if dg.Validations > 0 {
		fmt.Fprintf(&c, "运行了 %d 次测试/构建命令\n", dg.Validations)
	}
	if dg.LastAssistantText != "" {
		fmt.Fprintf(&c, "\n最近一次助手输出:\n%s\n", trunc(dg.LastAssistantText, 800))
	}
	if strings.TrimSpace(opt.PrevRecap) != "" {
		fmt.Fprintf(&c, "\n之前的进展纪要:\n%s\n", trunc(opt.PrevRecap, 1200))
	}

	body := trunc(c.String(), maxCtx)

	langName := "中文"
	if lang == "en" {
		langName = "English"
	}
	return fmt.Sprintf(`你在为一个非技术读者(产品经理/同事)总结一次 AI 编程会话的进展。读者看不懂代码,也看不到终端,只看这份总结。

下面是会话的结构化记录:
---
%s
---

请用%s输出,面向非技术读者,讲清楚"在做什么、做到哪了、卡在哪、下一步",避免文件名/函数名/工具名等术语。严格按下面格式输出,不要有多余文字:

===COCKPIT===
(2-3 句话,说明现在的状态:目标是什么、进展如何、是否卡住。给非技术读者看。)
===RECAP===
(markdown 进展纪要:用小标题和要点列出 已完成 / 进行中 / 待办,用业务语言而非技术细节。)
===MEMORY===
(每行一条需要长期记住的事实,格式 "kind: 内容"。kind 取值: important(重要部分) / gotcha(注意事项) / constraint(约束) / decision(决定) / resource(资源)。没有就留空。)
`, body, langName)
}

// parse splits the claude output on the section markers. Missing markers degrade
// gracefully: unmarked output becomes the recap.
func parse(out string) *Result {
	r := &Result{}
	sections := splitSections(out)
	r.Cockpit = strings.TrimSpace(sections["COCKPIT"])
	r.Recap = strings.TrimSpace(sections["RECAP"])
	for _, ln := range strings.Split(sections["MEMORY"], "\n") {
		ln = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "-"))
		if ln == "" {
			continue
		}
		kind, text := splitFact(ln)
		if text == "" {
			continue
		}
		r.Memory = append(r.Memory, MemoryFact{Kind: kind, Text: text})
	}
	// Fallback if the model ignored the format entirely.
	if r.Cockpit == "" && r.Recap == "" {
		r.Recap = strings.TrimSpace(out)
	}
	return r
}

// splitSections returns marker -> body for the three known markers.
func splitSections(out string) map[string]string {
	markers := []string{"COCKPIT", "RECAP", "MEMORY"}
	res := map[string]string{}
	type pos struct {
		name      string
		tagStart  int // index of the "===NAME===" tag
		bodyStart int // index just after the tag
	}
	var found []pos
	for _, m := range markers {
		tag := "===" + m + "==="
		if i := strings.Index(out, tag); i >= 0 {
			found = append(found, pos{name: m, tagStart: i, bodyStart: i + len(tag)})
		}
	}
	// Sort by tagStart ascending (insertion sort; tiny slice).
	for i := 1; i < len(found); i++ {
		for j := i; j > 0 && found[j-1].tagStart > found[j].tagStart; j-- {
			found[j-1], found[j] = found[j], found[j-1]
		}
	}
	for i, f := range found {
		end := len(out)
		if i+1 < len(found) {
			end = found[i+1].tagStart // body ends where the next tag begins
		}
		res[f.name] = out[f.bodyStart:end]
	}
	return res
}

func splitFact(s string) (kind, text string) {
	if i := strings.Index(s, ":"); i >= 0 {
		k := strings.ToLower(strings.TrimSpace(s[:i]))
		t := strings.TrimSpace(s[i+1:])
		switch k {
		case "important", "gotcha", "constraint", "decision", "resource", "fact":
			return k, t
		}
	}
	return "fact", s
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
