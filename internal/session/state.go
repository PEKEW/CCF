package session

import "time"

// Status / Phase well-known values.
const (
	StatusActive = "active"
	StatusEnded  = "ended"

	PhaseInit       = "init"
	PhasePlanning   = "planning"
	PhaseWorking    = "working"
	PhaseValidating = "validating"
	PhaseBlocked    = "blocked"
	PhaseDone       = "done"
)

// FeishuDocRef is a reference to a Feishu (or mock) document.
type FeishuDocRef struct {
	Name  string `json:"name"`
	Token string `json:"token"`
	URL   string `json:"url,omitempty"`
}

// DirtyState tracks pending changes not yet synced to Feishu.
type DirtyState struct {
	HasDirtyEvents      bool      `json:"has_dirty_events"`
	DirtyEventCount     int       `json:"dirty_event_count"`
	HasValidationUpdate bool      `json:"has_validation_update"`
	HasDecisionUpdate   bool      `json:"has_decision_update"`
	HasCompactUpdate    bool      `json:"has_compact_update"`
	HasHandoffUpdate    bool      `json:"has_handoff_update"`
	LastDirtyAt         time.Time `json:"last_dirty_at,omitempty"`
}

// SyncPolicy controls when buffered events get flushed to Feishu.
type SyncPolicy struct {
	Mode               string   `json:"mode"`
	IntervalMinutes    int      `json:"interval_minutes"`
	MinDirtyEvents     int      `json:"min_dirty_events"`
	MaxUnsyncedMinutes int      `json:"max_unsynced_minutes"`
	ImmediateEvents    []string `json:"immediate_events"`
}

// DefaultSyncPolicy returns the buffered default from the plan.
func DefaultSyncPolicy() SyncPolicy {
	return SyncPolicy{
		Mode:               "buffered",
		IntervalMinutes:    10,
		MinDirtyEvents:     5,
		MaxUnsyncedMinutes: 30,
		ImmediateEvents: []string{
			"first_prompt_title_generated",
			"plan_created",
			"validation_completed",
			"compact_completed",
			"blocked",
			"human_approval_required",
			"stop",
			"session_end",
		},
	}
}

// SessionState is the persisted local state for one managed session.
type SessionState struct {
	SessionID       string `json:"session_id"`
	ClaudeSessionID string `json:"claude_session_id,omitempty"`

	Status string `json:"status"`
	Phase  string `json:"phase"`

	Title              string `json:"title"`
	ProvisionalTitle   string `json:"provisional_title,omitempty"`
	FirstPromptSeen    bool   `json:"first_prompt_seen"`
	FirstPromptHash    string `json:"first_prompt_hash,omitempty"`
	FirstPromptSummary string `json:"first_prompt_summary,omitempty"`

	FeishuFolderToken string `json:"feishu_folder_token,omitempty"`
	FeishuFolderURL   string `json:"feishu_folder_url,omitempty"`

	Docs map[string]FeishuDocRef `json:"docs"`

	CWD       string `json:"cwd"`
	GitRemote string `json:"git_remote,omitempty"`
	GitBranch string `json:"git_branch,omitempty"`
	GitCommit string `json:"git_commit,omitempty"`
	GitDirty  bool   `json:"git_dirty"`

	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastSyncAt time.Time `json:"last_sync_at,omitempty"`

	Dirty      DirtyState `json:"dirty"`
	SyncPolicy SyncPolicy `json:"sync_policy"`
}
