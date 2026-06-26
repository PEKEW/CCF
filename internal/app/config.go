package app

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk config.yaml.
type Config struct {
	Feishu FeishuConfig `yaml:"feishu"`
	Sync   SyncConfig   `yaml:"sync"`
	Policy PolicyConfig `yaml:"policy"`
}

type FeishuConfig struct {
	AppID                string `yaml:"app_id"`
	AppSecret            string `yaml:"app_secret"`
	RootFolderToken      string `yaml:"root_folder_token"`
	RegistryBitableToken string `yaml:"registry_bitable_token"`
	BaseURL              string `yaml:"base_url"`
}

type SyncConfig struct {
	IntervalMinutes    int `yaml:"interval_minutes"`
	MinDirtyEvents     int `yaml:"min_dirty_events"`
	MaxUnsyncedMinutes int `yaml:"max_unsynced_minutes"`
}

type PolicyConfig struct {
	BlockTestWeakening            bool `yaml:"block_test_weakening"`
	RequireApprovalForTestChanges bool `yaml:"require_approval_for_test_changes"`
	RequireApprovalForCIChanges   bool `yaml:"require_approval_for_ci_changes"`
	RequireApprovalForDelete      bool `yaml:"require_approval_for_delete"`
	RequireApprovalForGitPush     bool `yaml:"require_approval_for_git_push"`
}

// DefaultConfig returns config with safe defaults.
func DefaultConfig() Config {
	return Config{
		Feishu: FeishuConfig{
			BaseURL: "https://open.feishu.cn",
		},
		Sync: SyncConfig{
			IntervalMinutes:    10,
			MinDirtyEvents:     5,
			MaxUnsyncedMinutes: 30,
		},
		Policy: PolicyConfig{
			BlockTestWeakening:            true,
			RequireApprovalForTestChanges: true,
			RequireApprovalForCIChanges:   true,
			RequireApprovalForDelete:      true,
			RequireApprovalForGitPush:     true,
		},
	}
}

// Home returns the ccfl state home directory.
// Honors CCFL_HOME, else ~/.cc-feishu-link.
func Home() (string, error) {
	if h := os.Getenv("CCFL_HOME"); h != "" {
		return h, nil
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".cc-feishu-link"), nil
}

// ConfigPath returns the path to config.yaml.
func ConfigPath() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "config.yaml"), nil
}

// SessionsDir returns the sessions root directory.
func SessionsDir() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "sessions"), nil
}

// LoadConfig reads config.yaml, applying defaults for unset numeric fields.
func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	p, err := ConfigPath()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // defaults
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Sync.IntervalMinutes == 0 {
		c.Sync.IntervalMinutes = 10
	}
	if c.Sync.MinDirtyEvents == 0 {
		c.Sync.MinDirtyEvents = 5
	}
	if c.Sync.MaxUnsyncedMinutes == 0 {
		c.Sync.MaxUnsyncedMinutes = 30
	}
	if c.Feishu.BaseURL == "" {
		c.Feishu.BaseURL = "https://open.feishu.cn"
	}
}

// Init writes a default config.yaml if absent. Returns the path and whether created.
func Init() (string, bool, error) {
	h, err := Home()
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(filepath.Join(h, "sessions"), 0o700); err != nil {
		return "", false, err
	}
	p := filepath.Join(h, "config.yaml")
	if _, err := os.Stat(p); err == nil {
		return p, false, nil // already exists, do not overwrite
	}
	b, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return "", false, err
	}
	return p, true, nil
}
