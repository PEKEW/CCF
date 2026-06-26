package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

// RunSessionStart creates (or reuses) a session and its Feishu folder/docs.
func (a *App) RunSessionStart(in *hooks.Input, out io.Writer) error {
	now := time.Now()
	if existing, _ := a.Store.FindByClaudeID(in.SessionID); existing != "" {
		a.notef("session-start: reusing %s for claude id %s", existing, in.SessionID)
		return hooks.Allow().Write(out)
	}

	cwd := in.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	st, err := a.Store.Create(in.SessionID, cwd, now)
	if err != nil {
		return err
	}

	if err := a.createFolderAndDocs(st, session.UntitledFolderName(now)); err != nil {
		return fmt.Errorf("create feishu folder/docs: %w", err)
	}
	if err := a.updateDoc(st, string(templates.KeyIndex), syncpkg.RenderIndex(st)); err != nil {
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
		return err
	}
	now := time.Now()
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))

	if !st.FirstPromptSeen {
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
		if err := a.updateDoc(st, string(templates.KeyIndex), syncpkg.RenderIndex(st)); err != nil {
			return err
		}
		if err := a.appendDoc(st, string(templates.KeyTaskPlan),
			"\n## Goal (draft)\n\n"+st.ProvisionalTitle+"\n"); err != nil {
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
		_ = a.appendDoc(st, string(templates.KeyValidationDecs),
			"\n### Validation "+now.Format(time.RFC3339)+"\n\n- Command: "+policy.Redact(f.Command)+"\n")
	}

	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{
		Kind: kind, HookEvent: "PostToolUse", ToolName: in.ToolName,
		Summary: summary, Sensitive: policy.IsSensitive(summary), SyncPriority: priority,
	})
	syncpkg.MarkEvent(st, now)
	return a.finishHook(st, trigger, out, hooks.Allow())
}

// RunStop flushes a checkpoint and updates handoff.
func (a *App) RunStop(in *hooks.Input, out io.Writer) error {
	st, err := a.resolveSession(in.SessionID)
	if err != nil {
		return err
	}
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "stop", HookEvent: "Stop", Summary: "stop", SyncPriority: syncpkg.PriorityImmediate})

	events, _ := buf.Since(st.LastSyncAt)
	if err := a.updateDoc(st, string(templates.KeyHandoff), syncpkg.RenderHandoff(st, events)); err != nil {
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
	buf := syncpkg.NewBuffer(a.Store.Dir(st.SessionID))
	_ = buf.Append(syncpkg.Event{Kind: "compact_pending", HookEvent: "PreCompact",
		Summary: "trigger=" + in.Trigger, SyncPriority: syncpkg.PriorityNormal})
	syncpkg.MarkCompact(st, time.Now())
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
	subject := tc.Command
	if subject == "" {
		subject = tc.FilePath
	}
	entry := fmt.Sprintf("\n### Decision %s\n\n- Context: %s on %q\n- Verdict: %s\n- Rule: %s\n- Reason: %s\n- Time: %s\n",
		hashString(verdict+subject), tc.Name, subject, verdict, risk.Rule, risk.Reason, time.Now().Format(time.RFC3339))
	_ = a.appendDoc(st, string(templates.KeyValidationDecs), entry)
	syncpkg.MarkDecision(st, time.Now())
	_ = a.Store.Save(st)
}
