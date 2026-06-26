package sync

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/peke/cc-feishu-link/internal/session"
)

// RenderCheckpoint builds a markdown checkpoint block from buffered events.
// Intended to be appended to 03_CHECKPOINTS.
func RenderCheckpoint(st *session.SessionState, events []Event, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Checkpoint %s\n\n", now.Format(time.RFC3339))

	fmt.Fprintf(&b, "### Summary\n\nPhase: %s. %d event(s) since last sync.\n\n",
		nonEmpty(st.Phase, "unknown"), len(events))

	files := touchedFiles(events)
	fmt.Fprintf(&b, "### What Changed\n\n")
	if len(files) == 0 {
		b.WriteString("- (no file edits recorded)\n")
	} else {
		for _, f := range files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Files Touched\n\n")
	if len(files) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, f := range files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Validation\n\n")
	val := filterKind(events, "validation")
	if len(val) == 0 {
		b.WriteString("- none recorded\n")
	} else {
		for _, e := range val {
			fmt.Fprintf(&b, "- %s\n", e.Summary)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Errors\n\n")
	errs := filterKind(events, "error")
	if len(errs) == 0 {
		b.WriteString("- none recorded\n")
	} else {
		for _, e := range errs {
			fmt.Fprintf(&b, "- %s\n", e.Summary)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Next Step\n\n- (to be filled by Claude)\n\n")
	return b.String()
}

func touchedFiles(events []Event) []string {
	set := map[string]struct{}{}
	for _, e := range events {
		if e.ToolName == "Edit" || e.ToolName == "Write" || e.ToolName == "MultiEdit" {
			if f := extractFile(e.Summary); f != "" {
				set[f] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// extractFile pulls a path token out of an event summary like "Edit foo/bar.go".
func extractFile(summary string) string {
	fields := strings.Fields(summary)
	if len(fields) >= 2 {
		return fields[len(fields)-1]
	}
	return ""
}

func filterKind(events []Event, kind string) []Event {
	var out []Event
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
