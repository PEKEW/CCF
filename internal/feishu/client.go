package feishu

import "context"

// FolderRef references a created folder.
type FolderRef struct {
	Token string
	URL   string
}

// DocRef references a created document.
type DocRef struct {
	Token string
	URL   string
}

// ArtifactRef references an uploaded file.
type ArtifactRef struct {
	Token string
	URL   string
}

// Client is the Feishu (or mock) backend used for session storage.
type Client interface {
	CreateSessionFolder(ctx context.Context, title, parentToken string) (*FolderRef, error)
	RenameFolder(ctx context.Context, folderToken, title string) error
	CreateDoc(ctx context.Context, folderToken, title, content string) (*DocRef, error)
	UpdateDoc(ctx context.Context, docToken, content string) error
	AppendDoc(ctx context.Context, docToken, content string) error
	// GetDocText returns the document's plain-text content (used to read back
	// human-authored notes from the memo doc).
	GetDocText(ctx context.Context, docToken string) (string, error)
	UploadArtifact(ctx context.Context, folderToken, path string) (*ArtifactRef, error)
}
