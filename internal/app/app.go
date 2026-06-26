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

// resolveSession finds the session for a hook input: by Claude id, else latest.
func (a *App) resolveSession(claudeID string) (*session.SessionState, error) {
	if claudeID != "" {
		id, err := a.Store.FindByClaudeID(claudeID)
		if err != nil {
			return nil, err
		}
		if id != "" {
			return a.Store.Load(id)
		}
	}
	id, err := a.Store.Latest()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("no session found; run session-start first")
	}
	return a.Store.Load(id)
}

// ctx is a short helper for a background context.
func ctx() context.Context { return context.Background() }
