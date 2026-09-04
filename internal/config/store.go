package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/local/nblink-companion/internal/model"
)

const SchemaVersion = 2

const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

type Settings struct {
	LaunchAtLogin          bool   `json:"launchAtLogin"`
	StartFavoritesOnLaunch bool   `json:"startFavoritesOnLaunch"`
	CredentialFile         string `json:"credentialFile,omitempty"`
	RDPLauncher            string `json:"rdpLauncher,omitempty"`
	VNCLauncher            string `json:"vncLauncher,omitempty"`
	RefreshMinutes         int    `json:"refreshMinutes"`
	ThemeMode              string `json:"themeMode,omitempty"`
}

type Config struct {
	Version  int                 `json:"version"`
	Settings Settings            `json:"settings"`
	Rules    []model.ForwardRule `json:"rules"`
}

func Default() Config {
	return Config{
		Version: SchemaVersion,
		Settings: Settings{
			StartFavoritesOnLaunch: true,
			RefreshMinutes:         5,
			ThemeMode:              ThemeSystem,
		},
	}
}

type configV1 struct {
	Version  int        `json:"version"`
	Settings settingsV1 `json:"settings"`
	Rules    []ruleV1   `json:"rules"`
}

type settingsV1 struct {
	LaunchAtLogin  bool   `json:"launchAtLogin"`
	AutoStartRules bool   `json:"autoStartRules"`
	CredentialFile string `json:"credentialFile,omitempty"`
	RDPLauncher    string `json:"rdpLauncher,omitempty"`
	VNCLauncher    string `json:"vncLauncher,omitempty"`
	RefreshMinutes int    `json:"refreshMinutes"`
}

type ruleV1 struct {
	EndpointKey string            `json:"endpointKey"`
	Name        string            `json:"name"`
	PeerID      string            `json:"peerId"`
	Host        string            `json:"host"`
	TargetPort  int               `json:"targetPort"`
	ListenPort  int               `json:"listenPort"`
	Kind        model.ServiceKind `json:"kind"`
	Enabled     bool              `json:"enabled"`
	WebScheme   string            `json:"webScheme,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nblink-companion", "config.json"), nil
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string { return s.path }

func (s *Store) BackupInvalid() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	backup := fmt.Sprintf("%s.invalid-%s", s.path, time.Now().Format("20060102-150405"))
	if err := os.Rename(s.path, backup); err != nil {
		return "", err
	}
	return backup, nil
}

func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if header.Version == 1 {
		return migrateV1(data)
	}
	if header.Version != SchemaVersion {
		return Config{}, fmt.Errorf("unsupported config version %d", header.Version)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Version != SchemaVersion {
		return Config{}, fmt.Errorf("unsupported config version %d", cfg.Version)
	}
	if cfg.Settings.RefreshMinutes <= 0 {
		cfg.Settings.RefreshMinutes = 5
	}
	cfg.Settings.ThemeMode = NormalizeThemeMode(cfg.Settings.ThemeMode)
	return cfg, Validate(cfg)
}

func migrateV1(data []byte) (Config, error) {
	var old configV1
	if err := json.Unmarshal(data, &old); err != nil {
		return Config{}, fmt.Errorf("decode config v1: %w", err)
	}
	cfg := Config{
		Version: SchemaVersion,
		Settings: Settings{
			LaunchAtLogin:          old.Settings.LaunchAtLogin,
			StartFavoritesOnLaunch: old.Settings.AutoStartRules,
			CredentialFile:         old.Settings.CredentialFile,
			RDPLauncher:            old.Settings.RDPLauncher,
			VNCLauncher:            old.Settings.VNCLauncher,
			RefreshMinutes:         old.Settings.RefreshMinutes,
		},
		Rules: make([]model.ForwardRule, 0, len(old.Rules)),
	}
	if cfg.Settings.RefreshMinutes <= 0 {
		cfg.Settings.RefreshMinutes = 5
	}
	cfg.Settings.ThemeMode = ThemeSystem
	for _, rule := range old.Rules {
		cfg.Rules = append(cfg.Rules, model.ForwardRule{
			EndpointKey: rule.EndpointKey,
			Name:        rule.Name,
			PeerID:      rule.PeerID,
			Host:        rule.Host,
			TargetPort:  rule.TargetPort,
			ListenPort:  rule.ListenPort,
			Kind:        rule.Kind,
			Favorite:    rule.Enabled,
			WebScheme:   rule.WebScheme,
		})
	}
	return cfg, Validate(cfg)
}

func (s *Store) Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.Version = SchemaVersion
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil && os.PathSeparator == '/' {
		return err
	}
	return replaceFile(tmp, s.path)
}

func Validate(cfg Config) error {
	if cfg.Settings.RefreshMinutes <= 0 {
		return errors.New("refresh minutes must be positive")
	}
	if cfg.Settings.ThemeMode != "" && NormalizeThemeMode(cfg.Settings.ThemeMode) != cfg.Settings.ThemeMode {
		return fmt.Errorf("invalid theme mode %q", cfg.Settings.ThemeMode)
	}
	seen := make(map[int]string)
	for _, rule := range cfg.Rules {
		if rule.ListenPort < 1024 || rule.ListenPort > 65535 {
			return fmt.Errorf("%s: listen port must be between 1024 and 65535", rule.Name)
		}
		if !rule.Endpoint().Valid() {
			return fmt.Errorf("%s: invalid endpoint", rule.Name)
		}
		if other, ok := seen[rule.ListenPort]; ok {
			return fmt.Errorf("listen port %d is shared by %s and %s", rule.ListenPort, other, rule.Name)
		}
		seen[rule.ListenPort] = rule.Name
	}
	return nil
}

func NormalizeThemeMode(value string) string {
	switch value {
	case ThemeLight, ThemeDark:
		return value
	default:
		return ThemeSystem
	}
}

func SortRules(rules []model.ForwardRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Favorite != rules[j].Favorite {
			return rules[i].Favorite
		}
		return rules[i].Name < rules[j].Name
	})
}
