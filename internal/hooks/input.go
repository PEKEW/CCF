package hooks

import (
	"encoding/json"
	"io"
)

// Input is the union of fields Claude Code passes to hooks on stdin.
// Unused fields for a given hook are simply left zero.
type Input struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`

	// UserPromptSubmit
	Prompt string `json:"prompt"`

	// PreToolUse / PostToolUse
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`

	// SessionStart
	Source string `json:"source"`

	// PreCompact
	Trigger string `json:"trigger"`

	// SessionEnd
	Reason string `json:"reason"`
}

// ParseInput reads and decodes a hook payload from r. An empty body yields a
// zero Input (useful for manual invocation/testing).
func ParseInput(r io.Reader) (*Input, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	in := &Input{}
	if len(b) == 0 {
		return in, nil
	}
	if err := json.Unmarshal(b, in); err != nil {
		return nil, err
	}
	return in, nil
}

// ToolInputFields extracts the fields ccfl cares about from tool_input.
type ToolInputFields struct {
	Command   string `json:"command"`
	FilePath  string `json:"file_path"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// ToolFields decodes tool_input into the known fields. Missing/invalid input
// yields a zero struct.
func (in *Input) ToolFields() ToolInputFields {
	var f ToolInputFields
	if len(in.ToolInput) > 0 {
		_ = json.Unmarshal(in.ToolInput, &f)
	}
	if f.FilePath == "" && f.Path != "" {
		f.FilePath = f.Path
	}
	return f
}
