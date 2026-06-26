package policy

// Action is the policy outcome for a tool call.
type Action string

const (
	ActionAllow           Action = "allow"
	ActionRequireApproval Action = "require_approval"
	ActionBlock           Action = "block"
)

// Options toggles which categories are enforced. Maps from app config.
type Options struct {
	BlockTestWeakening            bool
	RequireApprovalForTestChanges bool
	RequireApprovalForCIChanges   bool
	RequireApprovalForDelete      bool
	RequireApprovalForGitPush     bool
}

// DefaultOptions enables all protections.
func DefaultOptions() Options {
	return Options{
		BlockTestWeakening:            true,
		RequireApprovalForTestChanges: true,
		RequireApprovalForCIChanges:   true,
		RequireApprovalForDelete:      true,
		RequireApprovalForGitPush:     true,
	}
}

// ToolCall is the normalized subject of a risk evaluation.
type ToolCall struct {
	Name      string
	Command   string   // Bash
	FilePath  string   // Edit/Write
	FilePaths []string // MultiEdit / batched
}

// Risk is the evaluation result.
type Risk struct {
	Action Action
	Reason string
	Rule   string
}

// Evaluate applies deterministic risk rules to a tool call.
func Evaluate(opt Options, tc ToolCall) Risk {
	switch tc.Name {
	case "Bash":
		act, rule := classifyBash(tc.Command)
		switch act {
		case ActionBlock:
			return Risk{ActionBlock, "blocked dangerous command: " + rule, rule}
		case ActionRequireApproval:
			return Risk{ActionRequireApproval, "command needs approval: " + rule, rule}
		}
		return Risk{ActionAllow, "", ""}

	case "Edit", "Write", "MultiEdit":
		paths := tc.FilePaths
		if tc.FilePath != "" {
			paths = append(paths, tc.FilePath)
		}
		for _, p := range paths {
			if ok, what := IsTestIntegrityPath(p); ok {
				if (what == "CI configuration" && !opt.RequireApprovalForCIChanges) ||
					(what != "CI configuration" && !opt.RequireApprovalForTestChanges) {
					continue
				}
				return Risk{
					Action: ActionRequireApproval,
					Reason: "edit to " + what + " requires approval: " + p,
					Rule:   what,
				}
			}
		}
		return Risk{ActionAllow, "", ""}
	}
	return Risk{ActionAllow, "", ""}
}
