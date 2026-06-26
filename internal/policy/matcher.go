package policy

import "regexp"

// blockPatterns are Bash command shapes that are blocked outright (MVP).
var blockPatterns = []struct {
	re   *regexp.Regexp
	rule string
}{
	{regexp.MustCompile(`(?i)\brm\s+(-[a-z]*\s+)*`), "rm (file deletion)"},
	{regexp.MustCompile(`(?i)\bgit\s+push\b`), "git push"},
	{regexp.MustCompile(`(?i)\bdeploy\b`), "deploy"},
	{regexp.MustCompile(`(?i)\bsudo\b`), "sudo"},
	{regexp.MustCompile(`(?i)\bcurl\b[^|]*\|\s*(sudo\s+)?(sh|bash)\b`), "curl | sh"},
	{regexp.MustCompile(`(?i)\bwget\b[^|]*\|\s*(sudo\s+)?(sh|bash)\b`), "wget | sh"},
}

// approvalPatterns are Bash command shapes that require human approval (MVP).
var approvalPatterns = []struct {
	re   *regexp.Regexp
	rule string
}{
	{regexp.MustCompile(`(?i)\bchmod\b`), "chmod"},
	{regexp.MustCompile(`(?i)\bchown\b`), "chown"},
	{regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`), "git reset --hard"},
	{regexp.MustCompile(`(?i)\bgit\s+clean\b`), "git clean"},
}

// classifyBash returns (action, rule) for a Bash command string.
func classifyBash(cmd string) (Action, string) {
	for _, p := range blockPatterns {
		if p.re.MatchString(cmd) {
			return ActionBlock, p.rule
		}
	}
	for _, p := range approvalPatterns {
		if p.re.MatchString(cmd) {
			return ActionRequireApproval, p.rule
		}
	}
	return ActionAllow, ""
}
