package appservice

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/manager"
	"github.com/local/nblink-companion/internal/model"
)

type fakeProvider struct {
	block      <-chan struct{}
	probeCalls atomic.Int32
	services   []model.RemoteService
	targets    []model.WakeTarget
	wakeErr    error
	woke       model.WakeTarget
}

func (f *fakeProvider) wait(ctx context.Context) error {
	if f.block == nil {
		return nil
	}
	select {
	case <-f.block:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeProvider) Probe(ctx context.Context) (model.RuntimeInfo, error) {
	f.probeCalls.Add(1)
	if err := f.wait(ctx); err != nil {
		return model.RuntimeInfo{}, err
	}
	return model.RuntimeInfo{APIBase: "http://127.0.0.1:2080", Version: "3.8.2"}, nil
}

func (f *fakeProvider) ListServices(ctx context.Context) ([]model.RemoteService, error) {
	if err := f.wait(ctx); err != nil {
		return nil, err
	}
	return append([]model.RemoteService(nil), f.services...), nil
}

func (f *fakeProvider) ListWakeTargets(ctx context.Context) ([]model.WakeTarget, error) {
	if err := f.wait(ctx); err != nil {
		return nil, err
	}
	return append([]model.WakeTarget(nil), f.targets...), nil
}

func (f *fakeProvider) Wake(_ context.Context, target model.WakeTarget) error {
	f.woke = target
	return f.wakeErr
}

func (f *fakeProvider) SetCredentialFile(string) {}

type fakeManager struct {
	mu       sync.Mutex
	handler  func(manager.Status)
	running  map[string]bool
	statuses map[string]manager.Status
}

func newFakeManager() *fakeManager {
	return &fakeManager{running: map[string]bool{}, statuses: map[string]manager.Status{}}
}

func (f *fakeManager) SetStatusHandler(handler func(manager.Status)) { f.handler = handler }
func (f *fakeManager) StartFavorites([]model.ForwardRule) error      { return nil }
func (f *fakeManager) Start(rule model.ForwardRule) error {
	f.mu.Lock()
	f.running[rule.EndpointKey] = true
	f.mu.Unlock()
	return nil
}
func (f *fakeManager) Stop(key string) {
	f.mu.Lock()
	delete(f.running, key)
	f.mu.Unlock()
}
func (f *fakeManager) StopAll() {
	f.mu.Lock()
	f.running = map[string]bool{}
	f.mu.Unlock()
}
func (f *fakeManager) SyncRules([]model.ForwardRule) error { return nil }
func (f *fakeManager) Close()                              {}
func (f *fakeManager) Statuses() map[string]manager.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]manager.Status, len(f.statuses))
	for key, value := range f.statuses {
		result[key] = value
	}
	return result
}
func (f *fakeManager) Running(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[key]
}
func (f *fakeManager) publish(status manager.Status) {
	f.mu.Lock()
	f.statuses[status.EndpointKey] = status
	f.mu.Unlock()
	if f.handler != nil {
		f.handler(status)
	}
}

type fakePlatform struct {
	mu            sync.Mutex
	events        []AppSnapshot
	notifications []string
	clipboard     string
}

func (f *fakePlatform) Emit(name string, payload any) {
	if name != EventSnapshot {
		return
	}
	snapshot, ok := payload.(AppSnapshot)
	if !ok {
		return
	}
	f.mu.Lock()
	f.events = append(f.events, snapshot)
	f.mu.Unlock()
}
func (f *fakePlatform) Notify(title, message string) error {
	f.mu.Lock()
	f.notifications = append(f.notifications, title+":"+message)
	f.mu.Unlock()
	return nil
}
func (f *fakePlatform) SetClipboard(text string) error    { f.clipboard = text; return nil }
func (f *fakePlatform) ChooseFile(string) (string, error) { return "", nil }
func (f *fakePlatform) snapshots() []AppSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]AppSnapshot(nil), f.events...)
}

func testController(t *testing.T, provider *fakeProvider, ruleManager *fakeManager, platform *fakePlatform, cfg config.Config) *Controller {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	controller, start, cleanup := New(nil, store, provider, ruleManager, platform, cfg, "0.3.0", "")
	t.Cleanup(cleanup)
	start()
	return controller
}

func waitSnapshot(t *testing.T, platform *fakePlatform, predicate func(AppSnapshot) bool) AppSnapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, snapshot := range platform.snapshots() {
			if predicate(snapshot) {
				return snapshot
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for snapshot")
	return AppSnapshot{}
}

func TestSnapshotRevisionAndRefreshMerge(t *testing.T) {
	endpoint := model.Endpoint{PeerID: "peer", Host: "192.168.1.10", TargetPort: 443}
	provider := &fakeProvider{
		services: []model.RemoteService{{EndpointKey: endpoint.Key(), Name: "NAS", Endpoint: endpoint, Kind: model.ServiceKindWeb, WebScheme: "https"}},
		targets:  []model.WakeTarget{{Name: "PC", PeerID: "peer", MAC: "AA:BB:CC:DD:EE:FF"}},
	}
	platform := &fakePlatform{}
	controller := testController(t, provider, newFakeManager(), platform, config.Default())
	ready := waitSnapshot(t, platform, func(value AppSnapshot) bool { return value.SyncState == "ready" })
	if ready.Revision < 2 {
		t.Fatalf("expected revision to advance, got %d", ready.Revision)
	}
	if len(ready.Services) != 1 || ready.Services[0].Name != "NAS" {
		t.Fatalf("unexpected services: %#v", ready.Services)
	}
	if len(ready.WakeTargets) != 1 || ready.WakeTargets[0].TargetKey == "" {
		t.Fatalf("unexpected wake targets: %#v", ready.WakeTargets)
	}
	if bootstrap := controller.Bootstrap(); bootstrap.Revision != ready.Revision {
		t.Fatalf("bootstrap revision=%d, ready=%d", bootstrap.Revision, ready.Revision)
	}
}

func TestConcurrentRefreshIsCoalesced(t *testing.T) {
	block := make(chan struct{})
	provider := &fakeProvider{block: block}
	platform := &fakePlatform{}
	controller := testController(t, provider, newFakeManager(), platform, config.Default())
	deadline := time.Now().Add(time.Second)
	for provider.probeCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 20; i++ {
		controller.Refresh()
	}
	if got := provider.probeCalls.Load(); got != 1 {
		t.Fatalf("expected one in-flight refresh, got %d", got)
	}
	close(block)
	waitSnapshot(t, platform, func(value AppSnapshot) bool { return value.SyncState == "ready" })
}

func TestWakeOnlyAcceptsCurrentTargetKey(t *testing.T) {
	target := model.WakeTarget{Name: "PC", PeerID: "peer", MAC: "AA:BB:CC:DD:EE:FF"}
	provider := &fakeProvider{targets: []model.WakeTarget{target}}
	platform := &fakePlatform{}
	controller := testController(t, provider, newFakeManager(), platform, config.Default())
	ready := waitSnapshot(t, platform, func(value AppSnapshot) bool { return value.SyncState == "ready" })
	if err := controller.Wake("not-a-current-target"); err == nil {
		t.Fatal("expected unknown target to fail")
	}
	if err := controller.Wake(ready.WakeTargets[0].TargetKey); err != nil {
		t.Fatal(err)
	}
	if provider.woke.MAC != target.MAC {
		t.Fatalf("unexpected target: %#v", provider.woke)
	}
	provider.wakeErr = errors.New("wake failed")
	if err := controller.Wake(ready.WakeTargets[0].TargetKey); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestUpdateRuleValidatesAndPersists(t *testing.T) {
	const port = 23456
	cfg := config.Default()
	cfg.Rules = []model.ForwardRule{{EndpointKey: "key", Name: "SSH", PeerID: "peer", Host: "192.168.1.2", TargetPort: 22, ListenPort: 12022, Kind: model.ServiceKindTCP, Available: true}}
	controller := testController(t, &fakeProvider{}, newFakeManager(), &fakePlatform{}, cfg)
	controller.portAvailable = func(int) bool { return true }
	if err := controller.UpdateRule("key", RulePatch{ListenPort: 80, Kind: model.ServiceKindTCP}); err == nil {
		t.Fatal("expected privileged port rejection")
	}
	if err := controller.UpdateRule("key", RulePatch{ListenPort: port, Kind: model.ServiceKindWeb, WebScheme: "https"}); err != nil {
		t.Fatal(err)
	}
	rule, ok := controller.ruleByKey("key")
	if !ok || rule.ListenPort != port || rule.Kind != model.ServiceKindWeb || rule.WebScheme != "https" {
		t.Fatalf("unexpected rule: %#v", rule)
	}
}

func TestNotificationOnlyOnFailureTransitionAndRecovery(t *testing.T) {
	cfg := config.Default()
	cfg.Rules = []model.ForwardRule{{EndpointKey: "key", Name: "NAS", PeerID: "peer", Host: "192.168.1.2", TargetPort: 443, ListenPort: 18443, Kind: model.ServiceKindWeb}}
	ruleManager := newFakeManager()
	platform := &fakePlatform{}
	_ = testController(t, &fakeProvider{}, ruleManager, platform, cfg)
	ruleManager.publish(manager.Status{EndpointKey: "key", State: manager.StateReady, Message: "ready"})
	ruleManager.publish(manager.Status{EndpointKey: "key", State: manager.StateError, Message: "failed"})
	ruleManager.publish(manager.Status{EndpointKey: "key", State: manager.StateError, Message: "failed again"})
	ruleManager.publish(manager.Status{EndpointKey: "key", State: manager.StateReady, Message: "ready"})
	platform.mu.Lock()
	count := len(platform.notifications)
	platform.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected failure and recovery notifications, got %d", count)
	}
}
