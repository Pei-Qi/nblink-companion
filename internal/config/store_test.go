package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/local/nblink-companion/internal/model"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	cfg := Default()
	cfg.Settings.LaunchAtLogin = true
	cfg.Settings.StartFavoritesOnLaunch = false
	cfg.Settings.RefreshMinutes = 10
	cfg.Rules = []model.ForwardRule{{
		EndpointKey: "key",
		Name:        "db",
		PeerID:      "peer",
		Host:        "10.0.0.2",
		TargetPort:  3306,
		ListenPort:  33060,
		Kind:        model.ServiceKindTCP,
		Favorite:    true,
	}}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 || got.Rules[0].ListenPort != 33060 ||
		!got.Settings.LaunchAtLogin || got.Settings.StartFavoritesOnLaunch || got.Settings.RefreshMinutes != 10 {
		t.Fatalf("unexpected config: %+v", got)
	}
	cfg.Rules[0].ListenPort = 33061
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rules[0].ListenPort != 33061 {
		t.Fatalf("replacement save was not persisted: %+v", got)
	}
}

func TestBackupInvalidPreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte("{invalid json")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid config error")
	}
	backup, err := store.BackupInvalid()
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected backup path")
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("backup changed original data: %q", data)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid config should have moved to backup, stat error: %v", err)
	}
}

func TestStoreMigratesV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{
  "version": 1,
  "settings": {
    "launchAtLogin": true,
    "autoStartRules": true,
    "refreshMinutes": 3
  },
  "rules": [{
    "endpointKey": "key",
    "name": "web",
    "peerId": "peer",
    "host": "10.0.0.3",
    "targetPort": 80,
    "listenPort": 18080,
    "kind": "web",
    "enabled": true,
    "webScheme": "http"
  }]
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != SchemaVersion || !cfg.Settings.StartFavoritesOnLaunch || !cfg.Rules[0].Favorite {
		t.Fatalf("unexpected migrated config: %+v", cfg)
	}
}

func TestValidateRejectsDuplicatePorts(t *testing.T) {
	cfg := Default()
	base := model.ForwardRule{Name: "a", PeerID: "p", Host: "127.0.0.1", TargetPort: 80, ListenPort: 18080}
	other := base
	other.Name = "b"
	cfg.Rules = []model.ForwardRule{base, other}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected duplicate port error")
	}
}

func TestThemeModeDefaultsAndRoundTripsInSchemaV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	cfg := Default()
	if cfg.Version != 2 || cfg.Settings.ThemeMode != ThemeSystem {
		t.Fatalf("unexpected default theme config: %#v", cfg)
	}
	cfg.Settings.ThemeMode = ThemeDark
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 2 || loaded.Settings.ThemeMode != ThemeDark {
		t.Fatalf("theme did not round trip: %#v", loaded)
	}
}
