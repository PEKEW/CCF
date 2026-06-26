package app

import (
	"fmt"
	"io"

	syncpkg "github.com/peke/cc-feishu-link/internal/sync"
)

// RunStatus prints a summary of a session (latest if id == "").
func (a *App) RunStatus(id string, out io.Writer) error {
	if id == "" {
		var err error
		id, err = a.Store.Latest()
		if err != nil {
			return err
		}
		if id == "" {
			fmt.Fprintln(out, "no sessions found")
			return nil
		}
	}
	st, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	blocker := "-"
	if st.Phase == "blocked" {
		blocker = "session blocked"
	}
	fmt.Fprintf(out, "Session ID:        %s\n", st.SessionID)
	fmt.Fprintf(out, "Title:             %s\n", st.Title)
	fmt.Fprintf(out, "Feishu folder URL: %s\n", orDash(st.FeishuFolderURL))
	fmt.Fprintf(out, "Current phase:     %s\n", st.Phase)
	fmt.Fprintf(out, "Dirty events:      %d\n", st.Dirty.DirtyEventCount)
	last := "-"
	if !st.LastSyncAt.IsZero() {
		last = st.LastSyncAt.Format("2006-01-02 15:04:05")
	}
	fmt.Fprintf(out, "Last sync:         %s\n", last)
	fmt.Fprintf(out, "Current blocker:   %s\n", blocker)
	fmt.Fprintf(out, "Backend:           %s\n", a.Backend)
	return nil
}

// RunSync flushes a session. force ignores policy thresholds.
func (a *App) RunSync(id string, force bool, out io.Writer) error {
	if id == "" {
		var err error
		id, err = a.Store.Latest()
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("no session to sync")
		}
	}
	st, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	if force {
		if err := a.flush(st, "manual --force"); err != nil {
			return err
		}
		fmt.Fprintln(out, "synced (forced):", st.SessionID)
		return nil
	}
	d := syncpkg.EvaluateNow(st)
	if !d.Sync {
		// Manual sync always flushes; policy result is informational.
		if err := a.flush(st, "manual sync"); err != nil {
			return err
		}
	} else if err := a.flush(st, "manual sync: "+d.Reason); err != nil {
		return err
	}
	fmt.Fprintln(out, "synced:", st.SessionID)
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
