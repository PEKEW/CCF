package app

import (
	"github.com/PEKEW/CCF/internal/session"
	"github.com/PEKEW/CCF/internal/templates"
)

// createFolderAndDocs creates the Feishu folder and the 6 core docs, populating
// st.Docs / st.FeishuFolder*. In dry-run it fabricates placeholder tokens.
func (a *App) createFolderAndDocs(st *session.SessionState, folderTitle string) error {
	if a.Backend == BackendDry {
		st.FeishuFolderToken = "dry-folder"
		st.FeishuFolderURL = "dry://folder/" + st.SessionID
		for _, k := range templates.CoreDocs {
			st.Docs[string(k)] = session.FeishuDocRef{
				Name:  string(k),
				Token: "dry-" + string(k),
				URL:   "dry://doc/" + string(k),
			}
		}
		a.notef("[dry-run] would create folder %q with %d docs", folderTitle, len(templates.CoreDocs))
		return nil
	}

	fr, err := a.Client.CreateSessionFolder(ctx(), folderTitle, a.Cfg.Feishu.RootFolderToken)
	if err != nil {
		return err
	}
	st.FeishuFolderToken = fr.Token
	st.FeishuFolderURL = fr.URL

	for _, k := range templates.CoreDocs {
		dr, err := a.Client.CreateDoc(ctx(), fr.Token, string(k), templates.Initial(k))
		if err != nil {
			return err
		}
		st.Docs[string(k)] = session.FeishuDocRef{Name: string(k), Token: dr.Token, URL: dr.URL}
	}
	return nil
}

func (a *App) renameFolder(st *session.SessionState, title string) error {
	if a.Backend == BackendDry {
		a.notef("[dry-run] would rename folder -> %q", title)
		return nil
	}
	if st.FeishuFolderToken == "" {
		return nil
	}
	return a.Client.RenameFolder(ctx(), st.FeishuFolderToken, title)
}

// updateDoc replaces a doc's content (mock/dry); real backend appends per MVP.
func (a *App) updateDoc(st *session.SessionState, key, content string) error {
	d, ok := st.Docs[key]
	if !ok {
		return nil
	}
	if a.Backend == BackendDry {
		a.notef("[dry-run] would update doc %s (%d bytes)", key, len(content))
		return nil
	}
	return a.Client.UpdateDoc(ctx(), d.Token, content)
}

func (a *App) appendDoc(st *session.SessionState, key, content string) error {
	d, ok := st.Docs[key]
	if !ok {
		return nil
	}
	if a.Backend == BackendDry {
		a.notef("[dry-run] would append to doc %s (%d bytes)", key, len(content))
		return nil
	}
	return a.Client.AppendDoc(ctx(), d.Token, content)
}
