package appservice

import (
	"context"

	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/manager"
	"github.com/local/nblink-companion/internal/model"
)

const (
	EventSnapshot = "app:snapshot"
	EventToast    = "app:toast"
	EventNavigate = "ui:navigate"
)

type Provider interface {
	Probe(context.Context) (model.RuntimeInfo, error)
	ListServices(context.Context) ([]model.RemoteService, error)
	ListWakeTargets(context.Context) ([]model.WakeTarget, error)
	Wake(context.Context, model.WakeTarget) error
	SetCredentialFile(string)
}

type RuleManager interface {
	SetStatusHandler(func(manager.Status))
	StartFavorites([]model.ForwardRule) error
	Start(model.ForwardRule) error
	Stop(string)
	StopAll()
	SyncRules([]model.ForwardRule) error
	Close()
	Statuses() map[string]manager.Status
	Running(string) bool
}

type Platform interface {
	Emit(name string, payload any)
	Notify(title, message string) error
	SetClipboard(text string) error
	ChooseFile(kind string) (string, error)
}

type NodeSnapshot struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version"`
	APIBase   string `json:"apiBase"`
	Message   string `json:"message"`
}

type AppSummary struct {
	Total             int   `json:"total"`
	Running           int   `json:"running"`
	Favorites         int   `json:"favorites"`
	Errors            int   `json:"errors"`
	ActiveConnections int64 `json:"activeConnections"`
}

type ServiceSnapshot struct {
	EndpointKey       string            `json:"endpointKey"`
	Name              string            `json:"name"`
	Host              string            `json:"host"`
	TargetPort        int               `json:"targetPort"`
	TargetAddress     string            `json:"targetAddress"`
	ListenPort        int               `json:"listenPort"`
	LocalAddress      string            `json:"localAddress"`
	Kind              model.ServiceKind `json:"kind"`
	WebScheme         string            `json:"webScheme"`
	Favorite          bool              `json:"favorite"`
	Available         bool              `json:"available"`
	Running           bool              `json:"running"`
	State             manager.State     `json:"state"`
	StateLabel        string            `json:"stateLabel"`
	Message           string            `json:"message"`
	ActiveConnections int64             `json:"activeConnections"`
	CanOpen           bool              `json:"canOpen"`
}

type WakeTargetSnapshot struct {
	TargetKey string `json:"targetKey"`
	Name      string `json:"name"`
	MaskedMAC string `json:"maskedMAC"`
	Online    bool   `json:"online"`
}

type SettingsSnapshot struct {
	LaunchAtLogin          bool   `json:"launchAtLogin"`
	StartFavoritesOnLaunch bool   `json:"startFavoritesOnLaunch"`
	CredentialFile         string `json:"credentialFile"`
	RDPLauncher            string `json:"rdpLauncher"`
	VNCLauncher            string `json:"vncLauncher"`
	RefreshMinutes         int    `json:"refreshMinutes"`
	ThemeMode              string `json:"themeMode"`
}

type AppSnapshot struct {
	Revision     uint64               `json:"revision"`
	Version      string               `json:"version"`
	SyncState    string               `json:"syncState"`
	SyncMessage  string               `json:"syncMessage"`
	LastSyncedAt string               `json:"lastSyncedAt"`
	Node         NodeSnapshot         `json:"node"`
	Summary      AppSummary           `json:"summary"`
	Services     []ServiceSnapshot    `json:"services"`
	WakeTargets  []WakeTargetSnapshot `json:"wakeTargets"`
	Settings     SettingsSnapshot     `json:"settings"`
}

type RulePatch struct {
	ListenPort int               `json:"listenPort"`
	Kind       model.ServiceKind `json:"kind"`
	WebScheme  string            `json:"webScheme"`
}

type SettingsInput = SettingsSnapshot

type ToastEvent struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func settingsSnapshot(value config.Settings) SettingsSnapshot {
	return SettingsSnapshot{
		LaunchAtLogin: value.LaunchAtLogin, StartFavoritesOnLaunch: value.StartFavoritesOnLaunch,
		CredentialFile: value.CredentialFile, RDPLauncher: value.RDPLauncher, VNCLauncher: value.VNCLauncher,
		RefreshMinutes: value.RefreshMinutes, ThemeMode: config.NormalizeThemeMode(value.ThemeMode),
	}
}
