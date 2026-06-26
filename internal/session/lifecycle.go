package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Store resolves filesystem locations for sessions. SessionsRoot is the
// directory that holds one subdirectory per session.
type Store struct {
	SessionsRoot string
}

// NewStore returns a Store rooted at sessionsRoot.
func NewStore(sessionsRoot string) *Store {
	return &Store{SessionsRoot: sessionsRoot}
}

// Dir returns the directory for a given session id.
func (s *Store) Dir(id string) string {
	return filepath.Join(s.SessionsRoot, id)
}

func (s *Store) statePath(id string) string {
	return filepath.Join(s.Dir(id), "session.json")
}

// NewSessionID returns a random local session id.
func NewSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "ls-" + hex.EncodeToString(b[:])
}

// Create initializes and persists a new session.
func (s *Store) Create(claudeSessionID, cwd string, now time.Time) (*SessionState, error) {
	id := NewSessionID()
	st := &SessionState{
		SessionID:       id,
		ClaudeSessionID: claudeSessionID,
		Status:          StatusActive,
		Phase:           PhaseInit,
		Title:           "Untitled",
		Docs:            map[string]FeishuDocRef{},
		CWD:             cwd,
		CreatedAt:       now,
		UpdatedAt:       now,
		SyncPolicy:      DefaultSyncPolicy(),
	}
	st.fillGit(cwd)
	if err := s.ensureDirs(id); err != nil {
		return nil, err
	}
	if err := s.Save(st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) ensureDirs(id string) error {
	dir := s.Dir(id)
	for _, sub := range []string{"", "raw", filepath.Join("raw", "hook_payloads"), filepath.Join("raw", "tool_outputs")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return err
		}
	}
	return nil
}

// Save writes session.json atomically with 0600 perms.
func (s *Store) Save(st *SessionState) error {
	st.UpdatedAt = time.Now()
	if err := s.ensureDirs(st.SessionID); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	p := s.statePath(st.SessionID)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Load reads session.json for id.
func (s *Store) Load(id string) (*SessionState, error) {
	b, err := os.ReadFile(s.statePath(id))
	if err != nil {
		return nil, err
	}
	var st SessionState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("parse session.json: %w", err)
	}
	if st.Docs == nil {
		st.Docs = map[string]FeishuDocRef{}
	}
	return &st, nil
}

// FindByClaudeID returns the most recently updated session matching a Claude
// session id, or "" if none.
func (s *Store) FindByClaudeID(claudeID string) (string, error) {
	if claudeID == "" {
		return "", nil
	}
	entries, err := os.ReadDir(s.SessionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var best string
	var bestT time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		st, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		if st.ClaudeSessionID == claudeID && st.UpdatedAt.After(bestT) {
			best = st.SessionID
			bestT = st.UpdatedAt
		}
	}
	return best, nil
}

// Latest returns the most recently updated active session id, or "".
func (s *Store) Latest() (string, error) {
	entries, err := os.ReadDir(s.SessionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var best string
	var bestT time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		st, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		if st.UpdatedAt.After(bestT) {
			best = st.SessionID
			bestT = st.UpdatedAt
		}
	}
	return best, nil
}

// fillGit populates git fields by shelling out in cwd. Failures are ignored.
func (st *SessionState) fillGit(cwd string) {
	if cwd == "" {
		return
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = cwd
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	if run("rev-parse", "--is-inside-work-tree") != "true" {
		return
	}
	st.GitBranch = run("rev-parse", "--abbrev-ref", "HEAD")
	st.GitCommit = run("rev-parse", "--short", "HEAD")
	st.GitRemote = run("config", "--get", "remote.origin.url")
	st.GitDirty = run("status", "--porcelain") != ""
}
