package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PEKEW/CCF/internal/hooks"
	"github.com/PEKEW/CCF/internal/policy"
	"github.com/PEKEW/CCF/internal/session"
	syncpkg "github.com/PEKEW/CCF/internal/sync"
	"github.com/PEKEW/CCF/internal/templates"
)

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// injectedPrefixes are harness/system-injected wrappers that arrive on the
// UserPromptSubmit hook but are NOT the user's own text. Title generation must
// ignore them so the session is named from a genuine first prompt.
var injectedPrefixes = []string{
	"<task-notification",
	"<system-reminder",
	"<local-command",
	"<command-name",
	"<command-message",
	"<command-args",
	"<command-stdout",
	"<user-prompt-submit-hook",
}

// isInjectedPrompt reports whether a prompt is empty or a harness injection
// rather than real user input.
func isInjectedPrompt(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	low := strings.ToLower(t)
	for _, p := range injectedPrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// RunSessionStart creates (or reuses) a session and its Feishu folder/docs.
func (a *App) RunSessionStart(in *hooks.Input, out io.Writer) error {
	now := time.Now()
	if existing, _ := a.Store.FindByClaudeID(in.SessionID); existing != "" {
		// Resume: reuse the same session + Feishu docs. Reactivate if it was
		// ended, record the resume, and flush anything left dirty last time.
		st, err := a.Store.Load(existing)
		if err != nil {
			a.notef("session-start: reusing %s (load failed: %v)", existing, err)
			return hooks.Allow().Write(out)
		}
		st.Status = session.StatusActive
		buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
		_ = buf.Append(syncpkg.Event{
			Kind: "session_resume", HookEvent: "SessionStart",
			Summary: "resumed: " + st.SessionID, SyncPriority: syncpkg.PriorityImportant,
		})
		if err := a.Store.Save(st); err != nil {
			return err
		}
		// Flush only if last session left something unsynced; avoids writing an
		// empty checkpoint on every resume.
		if st.Dirty.Pending() {
			if err := a.flush(st, "session resume"); err != nil {
				a.notef("session-start: resume flush failed: %v", err)
			}
		}
		a.notef("session-start: resumed %s folder=%s", st.SessionID, st.FeishuFolderURL)
		return hooks.Context("SessionStart",
			"Feishu-managed session resumed: "+st.SessionID).Write(out)
	}

	cwd := in.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	st, err := a.Store.Create(in.SessionID, cwd, now)
	if err != nil {
		return err
	}
	st.DocLayout = session.LayoutV2 // new sessions use the v2 human-surface docs

	if err := a.createFolderAndDocs(st, session.UntitledFolderName(now)); err != nil {
		return fmt.Errorf("create feishu folder/docs: %w", err)
	}
	if err := a.updateDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
		return err
	}

	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{
		Kind: "session_start", HookEvent: "SessionStart",
		Summary: "session created: " + st.SessionID, SyncPriority: syncpkg.PriorityImportant,
	})
	if err := a.Store.Save(st); err != nil {
		return err
	}
	a.notef("session-start: created %s folder=%s", st.SessionID, st.FeishuFolderURL)
	return hooks.Context("SessionStart",
		"Feishu-managed session active: "+st.SessionID).Write(out)
}

// RunUserPromptSubmit handles title generation on first prompt, buffering after.
func (a *App) RunUserPromptSubmit(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		// No managed session for this id (e.g. session-start failed). Stay out
		// of the way rather than binding to an unrelated session.
		return hooks.Allow().Write(out)
	}
	now := time.Now()
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))

	if !st.FirstPromptSeen {
		// Harness/system injections (task-notification, system-reminder, slash
		// command echoes) are not the user's real first prompt. Buffer and wait
		// so the session title is generated from genuine user text.
		if isInjectedPrompt(in.Prompt) {
			_ = buf.Append(syncpkg.Event{
				Kind: "system_prompt", HookEvent: "UserPromptSubmit",
				Summary: "injected prompt skipped for title", SyncPriority: syncpkg.PriorityNormal,
			})
			return a.finishHook(st, "", out, hooks.Allow())
		}
		st.FirstPromptSeen = true
		st.ProvisionalTitle = session.ProvisionalTitle(in.Prompt)
		st.Title = st.ProvisionalTitle
		st.FirstPromptHash = hashString(in.Prompt)
		st.FirstPromptSummary = st.ProvisionalTitle
		st.Phase = session.PhasePlanning

		slug := session.Slug(in.Prompt)
		folderTitle := session.FolderName(now, slug)
		if err := a.renameFolder(st, folderTitle); err != nil {
			return err
		}
		// Seed the contract goal from the first prompt; Claude refines it later
		// via feishu_set_contract. Refresh cockpit + contract docs.
		st.Contract.Goal = st.ProvisionalTitle
		st.Contract.UpdatedAt = now
		syncpkg.MarkContract(st, now)
		syncpkg.MarkCockpit(st, now)
		if err := a.updateDoc(st, string(templates.KeyCockpit), syncpkg.RenderCockpit(st)); err != nil {
			return err
		}
		if err := a.updateDoc(st, string(templates.KeyContract), syncpkg.RenderContract(st)); err != nil {
			return err
		}

		_ = buf.Append(syncpkg.Event{
			Kind: "first_prompt_title_generated", HookEvent: "UserPromptSubmit",
			Summary: "title=" + st.Title + " slug=" + slug, SyncPriority: syncpkg.PriorityImmediate,
		})
		syncpkg.MarkEvent(st, now)
		a.notef("first prompt: title=%q slug=%q", st.Title, slug)
		return a.finishHook(st, "first_prompt_title_generated", out,
			hooks.Context("UserPromptSubmit", "Session titled: "+st.Title))
	}

	_ = buf.Append(syncpkg.Event{
		Kind: "user_prompt", HookEvent: "UserPromptSubmit",
		Summary: "prompt len=" + fmt.Sprint(len(in.Prompt)), SyncPriority: syncpkg.PriorityNormal,
	})
	syncpkg.MarkEvent(st, now)
	return a.finishHook(st, "", out, hooks.Allow())
}

// RunPreToolUse applies risk policy before a tool runs.
func (a *App) RunPreToolUse(in *hooks.Input, out io.Writer) error {
	f := in.ToolFields()
	tc := policy.ToolCall{Name: in.ToolName, Command: f.Command, FilePath: f.FilePath}
	risk := policy.Evaluate(a.Opts, tc)

	st, serr := a.resolveSession(in.SessionID)

	switch risk.Action {
	case policy.ActionBlock:
		if serr == nil {
			a.recordDecision(st, "BLOCK", risk, tc)
		}
		a.notef("pre-tool-use: BLOCK %s (%s)", in.ToolName, risk.Rule)
		return hooks.Deny(risk.Reason).Write(out)
	case policy.ActionRequireApproval:
		if serr == nil {
			a.recordDecision(st, "APPROVAL_REQUIRED", risk, tc)
		}
		a.notef("pre-tool-use: ASK %s (%s)", in.ToolName, risk.Rule)
		return hooks.Ask(risk.Reason).Write(out)
	}
	return hooks.Allow().Write(out)
}

// RunPostToolUse buffers a tool event and may trigger a sync.
func (a *App) RunPostToolUse(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		// No session yet: nothing to record, allow.
		return hooks.Allow().Write(out)
	}
	now := time.Now()
	f := in.ToolFields()

	summary := in.ToolName
	switch in.ToolName {
	case "Bash":
		summary = "Bash " + policy.Redact(f.Command)
	case "Edit", "Write", "MultiEdit":
		summary = in.ToolName + " " + f.FilePath
	}

	kind := "tool"
	priority := syncpkg.PriorityNormal
	trigger := ""
	if in.ToolName == "Bash" && policy.IsValidationCommand(f.Command) {
		kind = "validation"
		priority = syncpkg.PriorityImportant
		trigger = "validation_completed"
		syncpkg.MarkValidation(st, now)
		line := "Command: " + policy.Redact(f.Command)
		st.AppendValidation(session.LogEntry{Time: now, Kind: "validation", Text: line})
		_ = a.appendDoc(st, string(templates.KeyValDecs), syncpkg.RenderValidationEntry(now, line))
	}

	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{
		Kind: kind, HookEvent: "PostToolUse", ToolName: in.ToolName,
		Summary: summary, Sensitive: policy.IsSensitive(summary), SyncPriority: priority,
	})
	syncpkg.MarkEvent(st, now)
	return a.finishHook(st, trigger, out, hooks.Allow())
}

// RunStop updates the handoff doc and flushes.
func (a *App) RunStop(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		return hooks.Allow().Write(out)
	}
	now := time.Now()
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "stop", HookEvent: "Stop", Summary: "stop", SyncPriority: syncpkg.PriorityImmediate})

	syncpkg.MarkHandoff(st, now)
	if err := a.updateDoc(st, string(templates.KeyHandoff), syncpkg.RenderHandoffV2(st)); err != nil {
		return err
	}
	if err := a.flush(st, "stop hook"); err != nil {
		return err
	}
	return hooks.Allow().Write(out)
}

// RunPreCompact records a pre-compact snapshot marker.
func (a *App) RunPreCompact(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		return hooks.Allow().Write(out)
	}
	now := time.Now()
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "compact_pending", HookEvent: "PreCompact",
		Summary: "trigger=" + in.Trigger, SyncPriority: syncpkg.PriorityNormal})
	syncpkg.MarkCompact(st, now)
	// Compaction imminent: distill durable facts into Memory and push now.
	if a.distillMemory(st, now) {
		_ = a.updateDoc(st, string(templates.KeyMemory), syncpkg.RenderMemory(st))
	}
	return a.finishHook(st, "", out, hooks.Allow())
}

// RunPostCompact records compaction completion and refreshes active context.
func (a *App) RunPostCompact(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		return hooks.Allow().Write(out)
	}
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "compact_completed", HookEvent: "PostCompact",
		Summary: "compacted", SyncPriority: syncpkg.PriorityImmediate})
	syncpkg.MarkCompact(st, time.Now())
	return a.finishHook(st, "compact_completed", out, hooks.Allow())
}

// RunSessionEnd flushes a final checkpoint and marks the session ended.
func (a *App) RunSessionEnd(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		return hooks.Allow().Write(out)
	}
	st.Status = session.StatusEnded
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "session_end", HookEvent: "SessionEnd",
		Summary: "reason=" + in.Reason, SyncPriority: syncpkg.PriorityImmediate})
	if err := a.flush(st, "session end"); err != nil {
		return err
	}
	return hooks.Allow().Write(out)
}

// finishHook persists state via maybeSync then writes the hook output.
func (a *App) finishHook(st *session.SessionState, trigger string, out io.Writer, o hooks.Output) error {
	if err := a.maybeSync(st, trigger); err != nil {
		return err
	}
	return o.Write(out)
}

// recordDecision appends a decision entry to the validation/decisions doc.
func (a *App) recordDecision(st *session.SessionState, verdict string, risk policy.Risk, tc policy.ToolCall) {
	now := time.Now()
	subject := tc.Command
	if subject == "" {
		subject = tc.FilePath
	}
	ctx := fmt.Sprintf("%s on %q", tc.Name, subject)
	entry := syncpkg.RenderDecisionEntry(now, verdict, ctx, risk.Rule, risk.Reason)
	st.AppendDecision(session.LogEntry{Time: now, Kind: "decision", Text: verdict + ": " + ctx})
	_ = a.appendDoc(st, string(templates.KeyValDecs), entry)
	syncpkg.MarkDecision(st, now)
	_ = a.Store.Save(st)
}

// distillMemory copies durable facts (decisions, validations, constraints) from
// structured state into Memory, deduped by text. Deterministic — no invented
// prose. Returns true if any new item was added.
func (a *App) distillMemory(st *session.SessionState, now time.Time) bool {
	seen := map[string]bool{}
	for _, m := range st.Memory {
		seen[m.Text] = true
	}
	add := func(kind, text string) bool {
		if text == "" || seen[text] {
			return false
		}
		seen[text] = true
		st.Memory = append(st.Memory, session.MemoryItem{Time: now, Kind: kind, Text: text})
		return true
	}
	changed := false
	for _, c := range st.Contract.Constraints {
		changed = add("constraint", c) || changed
	}
	for _, d := range st.Decisions {
		changed = add("decision", d.Text) || changed
	}
	if changed {
		syncpkg.MarkMemory(st, now)
	}
	return changed
}
