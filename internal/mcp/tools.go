package mcp

// toolList returns the static MCP tool definitions with JSON Schemas. Tools
// appear to Claude as mcp__ccfl__<name>. Every tool accepts an optional
// session_id; omit it to target the active (latest) session.
func toolList() []map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	str := map[string]any{"type": "string"}
	strList := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	sess := map[string]any{"type": "string", "description": "optional; defaults to the active session"}

	criteria := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
				"done": map[string]any{"type": "boolean"},
			},
			"required": []string{"text"},
		},
	}

	tool := func(name, desc string, schema map[string]any) map[string]any {
		return map[string]any{"name": name, "description": desc, "inputSchema": schema}
	}

	return []map[string]any{
		tool("feishu_get_status",
			"Read the current Feishu-managed session status (goal, phase, health, blocker, criteria, doc URLs).",
			obj(map[string]any{"session_id": sess})),

		tool("feishu_set_contract",
			"Set/replace the Task Contract (01): goal, why, scope, acceptance criteria, constraints, risks. Call after the first prompt and an initial plan; update when scope changes.",
			obj(map[string]any{
				"session_id":          sess,
				"goal":                str,
				"why":                 str,
				"in_scope":            strList,
				"out_scope":           strList,
				"acceptance_criteria": criteria,
				"constraints":         strList,
				"risks":               strList,
			})),

		tool("feishu_update_cockpit",
			"Update the Cockpit (00): one-line summary of what's happening now, next step, blocker (\"\" clears it), health (green|yellow|red), progress note, optional phase.",
			obj(map[string]any{
				"session_id":    sess,
				"summary":       str,
				"next_step":     str,
				"blocker":       str,
				"health":        map[string]any{"type": "string", "enum": []string{"green", "yellow", "red"}},
				"progress_note": str,
				"phase":         str,
			})),

		tool("feishu_append_decision",
			"Append a meaningful decision to Validation & Decisions (03): what was chosen and why. Use for non-trivial choices/tradeoffs.",
			obj(map[string]any{
				"session_id": sess,
				"text":       str,
				"context":    str,
				"verdict":    str,
				"reason":     str,
			})),

		tool("feishu_append_validation",
			"Append a validation result to Validation & Decisions (03) in human terms, e.g. \"go test ./... -> PASS 142 tests\".",
			obj(map[string]any{"session_id": sess, "text": str}, "text")),

		tool("feishu_update_recap",
			"Replace the Recap (02) with a narrative progress summary: what's done, key decisions, current focus. Prose, NOT an event list. Call periodically and before stopping.",
			obj(map[string]any{"session_id": sess, "narrative": str}, "narrative")),

		tool("feishu_append_memory",
			"Append a durable fact to Memory (05) that must survive compaction. kind = constraint|decision|gotcha|resource|fact.",
			obj(map[string]any{
				"session_id": sess,
				"text":       str,
				"kind":       map[string]any{"type": "string", "enum": []string{"constraint", "decision", "gotcha", "resource", "fact"}},
			}, "text")),

		tool("feishu_update_handoff",
			"Update the Handoff (04): tried/done/remains/risks/how_to_resume/files_to_read. Call at stop / end of session.",
			obj(map[string]any{
				"session_id":    sess,
				"tried":         strList,
				"done":          strList,
				"remains":       strList,
				"risks":         strList,
				"how_to_resume": strList,
				"files_to_read": strList,
			})),
	}
}
