// Package transcript reads a Claude Code session transcript (JSONL) and
// distills it into a structured digest used to auto-populate the Feishu docs
// without requiring Claude to call any tools. Heuristic, deterministic, no LLM.
package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// Digest is the extracted summary of a transcript.
type Digest struct {
	Title             string         // last ai-title, if any
	Prompts           []string       // user asks (real text, injections filtered)
	EditedFiles       []string       // unique files touched by Edit/Write/MultiEdit
	ToolCounts        map[string]int // tool name -> count
	Validations       int            // test/build/lint commands run
	LastUserPrompt    string         // most recent real user ask
	LastAssistantText string         // most recent assistant prose block
}

type rawLine struct {
	Type    string          `json:"type"`
	AITitle string          `json:"aiTitle"`
	Message json.RawMessage `json:"message"`
}

type rawMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolInput struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	Command  string `json:"command"`
}

var injectedPrefixes = []string{
	"<task-notification", "<system-reminder", "<local-command",
	"<command-name", "<command-message", "<command-args", "<command-stdout",
	"<user-prompt-submit-hook", "caveat:",
}

func isInjected(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "" {
		return true
	}
	for _, p := range injectedPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func isValidationCmd(cmd string) bool {
	c := strings.ToLower(cmd)
	for _, k := range []string{"go test", "pytest", "npm test", "npm run test", "yarn test", "cargo test", "go vet", "go build", "make test", "jest", "golangci-lint", "ruff", "eslint"} {
		if strings.Contains(c, k) {
			return true
		}
	}
	return false
}

// Distill parses the transcript file at path. Missing/unreadable file yields an
// empty digest and no error (auto-distill is best-effort).
func Distill(path string) (*Digest, error) {
	d := &Digest{ToolCounts: map[string]int{}}
	if path == "" {
		return d, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return d, nil
	}
	defer f.Close()

	seenFiles := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024) // allow long lines

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rl rawLine
		if json.Unmarshal(line, &rl) != nil {
			continue
		}
		switch rl.Type {
		case "ai-title":
			if rl.AITitle != "" {
				d.Title = rl.AITitle
			}
		case "user":
			var m rawMsg
			if json.Unmarshal(rl.Message, &m) != nil || m.Role != "user" {
				continue
			}
			var s string
			if json.Unmarshal(m.Content, &s) != nil { // content is a string for real prompts
				continue
			}
			if isInjected(s) {
				continue
			}
			s = strings.TrimSpace(s)
			d.Prompts = append(d.Prompts, s)
			d.LastUserPrompt = s
		case "assistant":
			var m rawMsg
			if json.Unmarshal(rl.Message, &m) != nil {
				continue
			}
			var blocks []contentBlock
			if json.Unmarshal(m.Content, &blocks) != nil {
				continue
			}
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if t := strings.TrimSpace(b.Text); t != "" {
						d.LastAssistantText = t
					}
				case "tool_use":
					d.ToolCounts[b.Name]++
					var ti toolInput
					_ = json.Unmarshal(b.Input, &ti)
					fp := ti.FilePath
					if fp == "" {
						fp = ti.Path
					}
					switch b.Name {
					case "Edit", "Write", "MultiEdit":
						if fp != "" && !seenFiles[fp] {
							seenFiles[fp] = true
							d.EditedFiles = append(d.EditedFiles, fp)
						}
					case "Bash":
						if isValidationCmd(ti.Command) {
							d.Validations++
						}
					}
				}
			}
		}
	}
	return d, nil
}
