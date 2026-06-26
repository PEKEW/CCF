package hooks

import (
	"encoding/json"
	"io"
)

// Output is the JSON a hook prints to stdout for Claude Code.
type Output struct {
	Continue       *bool           `json:"continue,omitempty"`
	StopReason     string          `json:"stopReason,omitempty"`
	SuppressOutput bool            `json:"suppressOutput,omitempty"`
	SystemMessage  string          `json:"systemMessage,omitempty"`
	HookSpecific   *HookSpecific   `json:"hookSpecificOutput,omitempty"`
}

// HookSpecific carries PreToolUse permission decisions and extra context.
type HookSpecific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`       // allow|deny|ask
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

// Allow returns an empty output (implicit allow / continue).
func Allow() Output { return Output{} }

// Deny blocks a PreToolUse tool call.
func Deny(reason string) Output {
	return Output{
		HookSpecific: &HookSpecific{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}
}

// Ask asks the user to confirm a PreToolUse tool call.
func Ask(reason string) Output {
	return Output{
		HookSpecific: &HookSpecific{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "ask",
			PermissionDecisionReason: reason,
		},
	}
}

// Context attaches additional context for the named hook event.
func Context(event, ctx string) Output {
	return Output{
		HookSpecific: &HookSpecific{
			HookEventName:     event,
			AdditionalContext: ctx,
		},
	}
}

// Write encodes o as JSON to w.
func (o Output) Write(w io.Writer) error {
	return json.NewEncoder(w).Encode(o)
}
