package feishu

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MockClient emulates Feishu by writing markdown files under Root and tracking
// token->path mappings in a manifest, so separate ccfl processes (one per hook)
// share a consistent view.
type MockClient struct {
	Root string
	mu   sync.Mutex
}

type mockManifest struct {
	Seq     int               `json:"seq"`
	Folders map[string]string `json:"folders"` // token -> rel dir
	Docs    map[string]string `json:"docs"`     // token -> rel file
	Names   map[string]string `json:"names"`    // token -> display name
}

// NewMockClient returns a MockClient rooted at root.
func NewMockClient(root string) *MockClient {
	return &MockClient{Root: root}
}

func (m *MockClient) manifestPath() string { return filepath.Join(m.Root, "manifest.json") }

func (m *MockClient) load() (*mockManifest, error) {
	man := &mockManifest{Folders: map[string]string{}, Docs: map[string]string{}, Names: map[string]string{}}
	b, err := os.ReadFile(m.manifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return man, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, man); err != nil {
		return nil, err
	}
	if man.Folders == nil {
		man.Folders = map[string]string{}
	}
	if man.Docs == nil {
		man.Docs = map[string]string{}
	}
	if man.Names == nil {
		man.Names = map[string]string{}
	}
	return man, nil
}

func (m *MockClient) save(man *mockManifest) error {
	if err := os.MkdirAll(m.Root, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.manifestPath(), b, 0o600)
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "x"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func token(prefix string, seq int, title string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%d:%s", seq, title)))
	return fmt.Sprintf("%s_%d_%s", prefix, seq, hex.EncodeToString(h[:4]))
}

func (m *MockClient) CreateSessionFolder(_ context.Context, title, _ string) (*FolderRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	man, err := m.load()
	if err != nil {
		return nil, err
	}
	man.Seq++
	tok := token("fld", man.Seq, title)
	rel := tok + "_" + slugify(title)
	if err := os.MkdirAll(filepath.Join(m.Root, rel), 0o700); err != nil {
		return nil, err
	}
	man.Folders[tok] = rel
	man.Names[tok] = title
	if err := m.save(man); err != nil {
		return nil, err
	}
	return &FolderRef{Token: tok, URL: "mock://folder/" + tok}, nil
}

func (m *MockClient) RenameFolder(_ context.Context, folderToken, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	man, err := m.load()
	if err != nil {
		return err
	}
	if _, ok := man.Folders[folderToken]; !ok {
		return fmt.Errorf("mock: unknown folder %s", folderToken)
	}
	man.Names[folderToken] = title
	// Write a NAME file so the rename is visible on disk; keep dir path stable.
	if err := os.WriteFile(filepath.Join(m.Root, man.Folders[folderToken], "_FOLDER_NAME.txt"),
		[]byte(title+"\n"), 0o600); err != nil {
		return err
	}
	return m.save(man)
}

func (m *MockClient) CreateDoc(_ context.Context, folderToken, title, content string) (*DocRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	man, err := m.load()
	if err != nil {
		return nil, err
	}
	folderRel, ok := man.Folders[folderToken]
	if !ok {
		return nil, fmt.Errorf("mock: unknown folder %s", folderToken)
	}
	man.Seq++
	tok := token("doc", man.Seq, title)
	rel := filepath.Join(folderRel, slugify(title)+".md")
	if err := os.WriteFile(filepath.Join(m.Root, rel), []byte(content), 0o600); err != nil {
		return nil, err
	}
	man.Docs[tok] = rel
	man.Names[tok] = title
	if err := m.save(man); err != nil {
		return nil, err
	}
	return &DocRef{Token: tok, URL: "mock://doc/" + tok}, nil
}

func (m *MockClient) UpdateDoc(_ context.Context, docToken, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	man, err := m.load()
	if err != nil {
		return err
	}
	rel, ok := man.Docs[docToken]
	if !ok {
		return fmt.Errorf("mock: unknown doc %s", docToken)
	}
	return os.WriteFile(filepath.Join(m.Root, rel), []byte(content), 0o600)
}

func (m *MockClient) AppendDoc(_ context.Context, docToken, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	man, err := m.load()
	if err != nil {
		return err
	}
	rel, ok := man.Docs[docToken]
	if !ok {
		return fmt.Errorf("mock: unknown doc %s", docToken)
	}
	f, err := os.OpenFile(filepath.Join(m.Root, rel), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + content)
	return err
}

func (m *MockClient) UploadArtifact(_ context.Context, folderToken, path string) (*ArtifactRef, error) {
	return &ArtifactRef{Token: "art_mock", URL: "mock://artifact/" + slugify(filepath.Base(path))}, nil
}
