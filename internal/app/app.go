package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PEKEW/CCF/internal/feishu"
	"github.com/PEKEW/CCF/internal/policy"
	"github.com/PEKEW/CCF/internal/session"
)

// Backend selects the Feishu implementation.
type Backend string

const (
	BackendDry  Backend = "dry"
	BackendMock Backend = "mock"
	BackendReal Backend = "real"
)

// App holds shared dependencies for command handlers.
type App struct {
	Cfg     Config
	Store   *session.Store
	Client  feishu.Client
	Opts    policy.Options
	Backend Backend
	Log     *os.File // where human-readable notes go (stderr)
}

// New builds an App. dryRun forces BackendDry. Otherwise the backend is chosen
// by CCFL_BACKEND, else real if credentials exist, else mock.
func New(dryRun bool) (*App, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	sdir, err := SessionsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sdir, 0o700); err != nil {
		return nil, err
	}

	a := &App{
		Cfg:   cfg,
		Store: session.NewStore(sdir),
		Opts: policy.Options{
			BlockTestWeakening:            cfg.Policy.BlockTestWeakening,
			RequireApprovalForTestChanges: cfg.Policy.RequireApprovalForTestChanges,
			RequireApprovalForCIChanges:   cfg.Policy.RequireApprovalForCIChanges,
			RequireApprovalForDelete:      cfg.Policy.RequireApprovalForDelete,
			RequireApprovalForGitPush:     cfg.Policy.RequireApprovalForGitPush,
		},
		Log: os.Stderr,
	}

	switch {
	case dryRun:
		a.Backend = BackendDry
	default:
		switch Backend(os.Getenv("CCFL_BACKEND")) {
		case BackendMock:
			a.Backend = BackendMock
		case BackendReal:
			a.Backend = BackendReal
		default:
			if cfg.Feishu.AppID != "" && cfg.Feishu.AppSecret != "" {
				a.Backend = BackendReal
			} else {
				a.Backend = BackendMock
			}
		}
	}

	switch a.Backend {
	case BackendMock:
		a.Client = feishu.NewMockClient(a.mockDir())
	case BackendReal:
		a.Client = feishu.NewRealClient(cfg.Feishu.AppID, cfg.Feishu.AppSecret, cfg.Feishu.BaseURL)
	case BackendDry:
		a.Client = nil // calls are skipped
	}
	return a, nil
}

func (a *App) mockDir() string {
	if d := os.Getenv("CCFL_MOCK_DIR"); d != "" {
		return d
	}
	h, _ := Home()
	return filepath.Join(h, ".mock-feishu")
}

func (a *App) notef(format string, args ...any) {
	fmt.Fprintf(a.Log, "[ccfl] "+format+"\n", args...)
}

// errNoSession is returned when no managed session matches the request. Hook
// handlers treat it as "stay out of the way" (allow), not a hard failure.
var errNoSession = fmt.Errorf("no session found; run session-start first")

// resolveSession finds the session for a hook input.
//
// When a Claude id is given, it must match exactly — we do NOT fall back to the
// latest session, otherwise a hook whose session-start failed (e.g. folder
// creation forbidden) would bind to and corrupt an unrelated session. The
// latest-session fallback applies only to CLI calls that pass no id (status/sync).
func (a *App) resolveSession(claudeID string) (*session.SessionState, error) {
	if claudeID != "" {
		id, err := a.Store.FindByClaudeID(claudeID)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, errNoSession
		}
		return a.Store.Load(id)
	}
	id, err := a.Store.Latest()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errNoSession
	}
	return a.Store.Load(id)
}

// ResolveForMCP resolves the session an MCP tool should act on:
// explicit idArg (Claude id or local id) → CCFL_SESSION env → latest.
func (a *App) ResolveForMCP(idArg string) (*session.SessionState, error) {
	try := func(id string) (*session.SessionState, error) {
		if id == "" {
			return nil, nil
		}
		if local, _ := a.Store.FindByClaudeID(id); local != "" {
			return a.Store.Load(local)
		}
		if st, err := a.Store.Load(id); err == nil {
			return st, nil
		}
		return nil, nil
	}
	if st, err := try(idArg); err != nil {
		return nil, err
	} else if st != nil {
		return st, nil
	}
	if st, err := try(os.Getenv("CCFL_SESSION")); err != nil {
		return nil, err
	} else if st != nil {
		return st, nil
	}
	id, err := a.Store.Latest()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errNoSession
	}
	return a.Store.Load(id)
}

// PushDoc renders+writes one doc for the current backend (replace).
func (a *App) PushDoc(st *session.SessionState, key string, content string) error {
	return a.updateDoc(st, key, content)
}

// AppendToDoc appends content to one doc for the current backend.
func (a *App) AppendToDoc(st *session.SessionState, key string, content string) error {
	return a.appendDoc(st, key, content)
}

// SaveSession persists session state.
func (a *App) SaveSession(st *session.SessionState) error { return a.Store.Save(st) }

// ctx is a short helper for a background context.
func ctx() context.Context { return context.Background() }
