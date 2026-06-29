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

// Criterion is one acceptance-criteria checkbox in the task contract.
type Criterion struct {
	Text string `json:"text"`
	Done bool   `json:"done,omitempty"`
}

// Contract backs 01_TASK_CONTRACT. Authored by Claude via the MCP
// feishu_set_contract tool; ccfl only renders it.
type Contract struct {
	Goal               string      `json:"goal,omitempty"`
	Why                string      `json:"why,omitempty"`
	InScope            []string    `json:"in_scope,omitempty"`
	OutScope           []string    `json:"out_scope,omitempty"`
	AcceptanceCriteria []Criterion `json:"acceptance_criteria,omitempty"`
	Constraints        []string    `json:"constraints,omitempty"` // tests/CI/security must-not-break
	Risks              []string    `json:"risks,omitempty"`
	UpdatedAt          time.Time   `json:"updated_at,omitempty"`
}

// Cockpit backs 00_COCKPIT. Authored via feishu_update_cockpit.
type Cockpit struct {
	Summary      string `json:"summary,omitempty"`       // what's happening now (1-2 sentences)
	NextStep     string `json:"next_step,omitempty"`     // the immediate next action
	Blocker      string `json:"blocker,omitempty"`       // "" => no blocker
	Health       string `json:"health,omitempty"`        // green|yellow|red
	ProgressNote string `json:"progress_note,omitempty"` // e.g. "3/5 criteria done"
}

// MemoryItem is one durable fact in 05_MEMORY that must survive compaction.
type MemoryItem struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"` // constraint|decision|gotcha|resource|fact
	Text string    `json:"text"`
}

// Handoff backs 04_HANDOFF (replace). Authored via feishu_update_handoff.
type Handoff struct {
	Tried       []string  `json:"tried,omitempty"`
	Done        []string  `json:"done,omitempty"`
	Remains     []string  `json:"remains,omitempty"`
	Risks       []string  `json:"risks,omitempty"`
	HowToResume []string  `json:"how_to_resume,omitempty"`
	FilesToRead []string  `json:"files_to_read,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// LogEntry is a bounded in-state mirror of an entry already appended to the
// Validation & Decisions doc. The doc holds full history; state keeps a tail.
type LogEntry struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"` // validation|decision
	Text string    `json:"text"`
}

// DirtyState tracks pending changes not yet synced to Feishu.
type DirtyState struct {
	HasDirtyEvents      bool      `json:"has_dirty_events"`
	DirtyEventCount     int       `json:"dirty_event_count"`
	HasValidationUpdate bool      `json:"has_validation_update"`
	HasDecisionUpdate   bool      `json:"has_decision_update"`
	HasCompactUpdate    bool      `json:"has_compact_update"`
	HasHandoffUpdate    bool      `json:"has_handoff_update"`
	HasContractUpdate   bool      `json:"has_contract_update,omitempty"`
	HasCockpitUpdate    bool      `json:"has_cockpit_update,omitempty"`
	HasRecapUpdate      bool      `json:"has_recap_update,omitempty"`
	HasMemoryUpdate     bool      `json:"has_memory_update,omitempty"`
	LastDirtyAt         time.Time `json:"last_dirty_at,omitempty"`
}

// Pending reports whether any unsynced change is waiting to be flushed.
func (d DirtyState) Pending() bool {
	return d.HasDirtyEvents || d.HasValidationUpdate || d.HasDecisionUpdate ||
		d.HasCompactUpdate || d.HasHandoffUpdate || d.HasContractUpdate ||
		d.HasCockpitUpdate || d.HasRecapUpdate || d.HasMemoryUpdate
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

	// DocLayout selects the document generation: "" / "legacy" = original 6
	// docs (event-log style); "v2" = cockpit/contract/handoff human surface.
	DocLayout string `json:"doc_layout,omitempty"`

	// Human-authored doc backing fields (set by MCP tools, rendered by ccfl).
	Contract       Contract     `json:"contract,omitempty"`
	Cockpit        Cockpit      `json:"cockpit,omitempty"`
	RecapNarrative string       `json:"recap_narrative,omitempty"`
	Memory         []MemoryItem `json:"memory,omitempty"`
	Handoff        Handoff      `json:"handoff,omitempty"`

	// Bounded tails mirroring the append-only Validation & Decisions doc.
	Decisions   []LogEntry `json:"decisions,omitempty"`
	Validations []LogEntry `json:"validations,omitempty"`
}

// LayoutV2 is the doc-layout marker for the cockpit/contract/handoff doc set.
const LayoutV2 = "v2"

// IsV2 reports whether this session uses the v2 (human-surface) doc layout.
func (s *SessionState) IsV2() bool { return s.DocLayout == LayoutV2 }

// logCap bounds the in-state validation/decision tails (full history lives in
// the Feishu doc).
const logCap = 50

// AppendDecision records a decision entry in the bounded in-state tail.
func (s *SessionState) AppendDecision(e LogEntry) {
	s.Decisions = appendCapped(s.Decisions, e)
}

// AppendValidation records a validation entry in the bounded in-state tail.
func (s *SessionState) AppendValidation(e LogEntry) {
	s.Validations = appendCapped(s.Validations, e)
}

func appendCapped(xs []LogEntry, e LogEntry) []LogEntry {
	xs = append(xs, e)
	if len(xs) > logCap {
		xs = xs[len(xs)-logCap:]
	}
	return xs
}
