// Package mcp implements a minimal stdio JSON-RPC 2.0 MCP server exposing
// ccfl's Feishu doc-authoring tools. Wire format is newline-delimited JSON
// (one request/response object per line), per the MCP stdio transport.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PEKEW/CCF/internal/app"
	"github.com/PEKEW/CCF/internal/session"
	syncpkg "github.com/PEKEW/CCF/internal/sync"
	"github.com/PEKEW/CCF/internal/templates"
)

const protocolVersion = "2025-06-18"

// Server holds the ccfl App used to fulfil tool calls.
type Server struct {
	app *app.App
}

// NewServer builds an MCP server over an existing App.
func NewServer(a *app.App) *Server { return &Server{app: a} }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests from r and writes responses to w until EOF.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for {
		line, err := br.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			s.handleLine(line, bw)
			bw.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (s *Server) handleLine(line []byte, w io.Writer) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		writeResp(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	// Notifications (no id) get no response.
	switch req.Method {
	case "notifications/initialized", "notifications/cancelled":
		return
	}
	resp := s.dispatch(req)
	if resp != nil {
		writeResp(w, *resp)
	}
}

func (s *Server) dispatch(req rpcRequest) *rpcResponse {
	base := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "ccfl", "version": "1.0.0"},
		}
	case "ping":
		base.Result = map[string]any{}
	case "tools/list":
		base.Result = map[string]any{"tools": toolList()}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = &rpcError{Code: -32602, Message: "invalid params"}
			return &base
		}
		text, err := s.callTool(p.Name, p.Arguments)
		if err != nil && text == "" {
			// Distinguish unknown tool (protocol error) from tool failure.
			if strings.HasPrefix(err.Error(), "unknown tool") {
				base.Error = &rpcError{Code: -32602, Message: err.Error()}
				return &base
			}
		}
		base.Result = toolResult(textOrErr(text, err), err != nil)
	default:
		base.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return &base
}

func textOrErr(text string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return text
}

func writeResp(w io.Writer, resp rpcResponse) {
	b, _ := json.Marshal(resp)
	w.Write(b)
	w.Write([]byte("\n"))
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

// --- tool dispatch ---

// withSession resolves the target session, enforces the v2 layout, runs fn,
// saves state, and returns fn's confirmation text.
func (s *Server) withSession(sessionID string, fn func(st *session.SessionState, now time.Time) (string, error)) (string, error) {
	st, err := s.app.ResolveForMCP(sessionID)
	if err != nil {
		return "", err
	}
	if !st.IsV2() {
		return "", fmt.Errorf("session %s uses the legacy doc layout; MCP authoring is only supported for v2 sessions", st.SessionID)
	}
	now := time.Now()
	msg, err := fn(st, now)
	if err != nil {
		return "", err
	}
	if err := s.app.SaveSession(st); err != nil {
		return "", err
	}
	return msg, nil
}

func (s *Server) callTool(name string, args json.RawMessage) (string, error) {
	switch name {
	case "feishu_get_status":
		return s.toolGetStatus(args)
	case "feishu_set_contract":
		return s.toolSetContract(args)
	case "feishu_update_cockpit":
		return s.toolUpdateCockpit(args)
	case "feishu_append_decision":
		return s.toolAppendDecision(args)
	case "feishu_append_validation":
		return s.toolAppendValidation(args)
	case "feishu_update_recap":
		return s.toolUpdateRecap(args)
	case "feishu_append_memory":
		return s.toolAppendMemory(args)
	case "feishu_update_handoff":
		return s.toolUpdateHandoff(args)
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

type sessionArg struct {
	SessionID string `json:"session_id"`
}

func (s *Server) toolGetStatus(args json.RawMessage) (string, error) {
	var a sessionArg
	_ = json.Unmarshal(args, &a)
	st, err := s.app.ResolveForMCP(a.SessionID)
	if err != nil {
		return "", err
	}
	out := map[string]any{
		"session_id":   st.SessionID,
		"title":        st.Title,
		"status":       st.Status,
		"phase":        st.Phase,
		"doc_layout":   st.DocLayout,
		"health":       st.Cockpit.Health,
		"blocker":      st.Cockpit.Blocker,
		"next_step":    st.Cockpit.NextStep,
		"goal":         st.Contract.Goal,
		"criteria":     st.Contract.AcceptanceCriteria,
		"folder_url":   st.FeishuFolderURL,
		"memory_items": len(st.Memory),
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b), nil
}

func (s *Server) toolSetContract(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Goal               string              `json:"goal"`
		Why                string              `json:"why"`
		InScope            []string            `json:"in_scope"`
		OutScope           []string            `json:"out_scope"`
		AcceptanceCriteria []session.Criterion `json:"acceptance_criteria"`
		Constraints        []string            `json:"constraints"`
		Risks              []string            `json:"risks"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		c := &st.Contract
		if a.Goal != "" {
			c.Goal = a.Goal
		}
		if a.Why != "" {
			c.Why = a.Why
		}
		if a.InScope != nil {
			c.InScope = a.InScope
		}
		if a.OutScope != nil {
			c.OutScope = a.OutScope
		}
		if a.AcceptanceCriteria != nil {
			c.AcceptanceCriteria = a.AcceptanceCriteria
		}
		if a.Constraints != nil {
			c.Constraints = a.Constraints
		}
		if a.Risks != nil {
			c.Risks = a.Risks
		}
		c.UpdatedAt = now
		syncpkg.MarkContract(st, now)
		syncpkg.MarkCockpit(st, now)
		if err := s.app.PushDoc(st, string(templates.KeyContract), syncpkg.RenderContract(st)); err != nil {
			return "", err
		}
		if err := s.app.PushDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
			return "", err
		}
		return "contract updated", nil
	})
}

func (s *Server) toolUpdateCockpit(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Summary      string  `json:"summary"`
		NextStep     string  `json:"next_step"`
		Blocker      *string `json:"blocker"`
		Health       string  `json:"health"`
		ProgressNote string  `json:"progress_note"`
		Phase        string  `json:"phase"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		if a.Summary != "" {
			st.Cockpit.Summary = a.Summary
		}
		if a.NextStep != "" {
			st.Cockpit.NextStep = a.NextStep
		}
		if a.Blocker != nil { // explicit, allows clearing with ""
			st.Cockpit.Blocker = *a.Blocker
		}
		if a.Health != "" {
			st.Cockpit.Health = a.Health
		}
		if a.ProgressNote != "" {
			st.Cockpit.ProgressNote = a.ProgressNote
		}
		if a.Phase != "" {
			st.Phase = a.Phase
		}
		syncpkg.MarkCockpit(st, now)
		if err := s.app.PushDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
			return "", err
		}
		return "cockpit updated", nil
	})
}

func (s *Server) toolAppendDecision(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Text    string `json:"text"`
		Context string `json:"context"`
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Text == "" && a.Verdict == "" && a.Reason == "" {
		return "", fmt.Errorf("decision requires text or verdict/reason")
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		verdict := a.Verdict
		if verdict == "" {
			verdict = "DECISION"
		}
		ctx := a.Context
		reason := a.Reason
		if a.Text != "" && reason == "" {
			reason = a.Text
		}
		entry := syncpkg.RenderDecisionEntry(now, verdict, ctx, "", reason)
		st.AppendDecision(session.LogEntry{Time: now, Kind: "decision", Text: strings.TrimSpace(verdict + ": " + firstNonEmpty(a.Text, reason))})
		syncpkg.MarkDecision(st, now)
		if err := s.app.AppendToDoc(st, string(templates.KeyValDecs), entry); err != nil {
			return "", err
		}
		return "decision recorded", nil
	})
}

func (s *Server) toolAppendValidation(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Text == "" {
		return "", fmt.Errorf("validation requires text")
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		st.AppendValidation(session.LogEntry{Time: now, Kind: "validation", Text: a.Text})
		syncpkg.MarkValidation(st, now)
		if err := s.app.AppendToDoc(st, string(templates.KeyValDecs), syncpkg.RenderValidationEntry(now, a.Text)); err != nil {
			return "", err
		}
		return "validation recorded", nil
	})
}

func (s *Server) toolUpdateRecap(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Narrative string `json:"narrative"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Narrative) == "" {
		return "", fmt.Errorf("recap requires narrative")
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		st.RecapNarrative = a.Narrative
		syncpkg.MarkRecap(st, now)
		if err := s.app.PushDoc(st, string(templates.KeyRecap), syncpkg.RenderRecap(st)); err != nil {
			return "", err
		}
		if err := s.app.PushDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
			return "", err
		}
		return "recap updated", nil
	})
}

func (s *Server) toolAppendMemory(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Text string `json:"text"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Text == "" {
		return "", fmt.Errorf("memory requires text")
	}
	kind := a.Kind
	if kind == "" {
		kind = "fact"
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		st.Memory = append(st.Memory, session.MemoryItem{Time: now, Kind: kind, Text: a.Text})
		syncpkg.MarkMemory(st, now)
		if err := s.app.PushDoc(st, string(templates.KeyMemory), syncpkg.RenderMemory(st)); err != nil {
			return "", err
		}
		return "memory appended", nil
	})
}

func (s *Server) toolUpdateHandoff(args json.RawMessage) (string, error) {
	var a struct {
		sessionArg
		Tried       []string `json:"tried"`
		Done        []string `json:"done"`
		Remains     []string `json:"remains"`
		Risks       []string `json:"risks"`
		HowToResume []string `json:"how_to_resume"`
		FilesToRead []string `json:"files_to_read"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	return s.withSession(a.SessionID, func(st *session.SessionState, now time.Time) (string, error) {
		h := &st.Handoff
		if a.Tried != nil {
			h.Tried = a.Tried
		}
		if a.Done != nil {
			h.Done = a.Done
		}
		if a.Remains != nil {
			h.Remains = a.Remains
		}
		if a.Risks != nil {
			h.Risks = a.Risks
		}
		if a.HowToResume != nil {
			h.HowToResume = a.HowToResume
		}
		if a.FilesToRead != nil {
			h.FilesToRead = a.FilesToRead
		}
		h.UpdatedAt = now
		syncpkg.MarkHandoff(st, now)
		if err := s.app.PushDoc(st, string(templates.KeyHandoff), syncpkg.RenderHandoffV2(st)); err != nil {
			return "", err
		}
		return "handoff updated", nil
	})
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}
