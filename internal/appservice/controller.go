package appservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/nblink-companion/internal/autostart"
	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/manager"
	"github.com/local/nblink-companion/internal/model"
)

type notificationState struct {
	hadReady bool
	failed   bool
}

type Controller struct {
	logger     *slog.Logger
	store      *config.Store
	provider   Provider
	supervisor RuleManager
	platform   Platform
	version    string

	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.RWMutex
	cfg           config.Config
	wakeTargets   []model.WakeTarget
	runtime       model.RuntimeInfo
	nodeErr       error
	revision      uint64
	syncState     string
	syncMessage   string
	lastSyncedAt  time.Time
	refreshing    bool
	notifications map[string]notificationState
	startupWarn   string
	portAvailable func(int) bool
	setAutostart  func(bool, string) error
}

func New(
	logger *slog.Logger,
	store *config.Store,
	provider Provider,
	supervisor RuleManager,
	platform Platform,
	cfg config.Config,
	version string,
	startupWarning string,
) (*Controller, func(), func()) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg.Settings.ThemeMode = config.NormalizeThemeMode(cfg.Settings.ThemeMode)
	controller := &Controller{
		logger: logger, store: store, provider: provider, supervisor: supervisor, platform: platform,
		cfg: cfg, version: version, ctx: ctx, cancel: cancel, syncState: "idle",
		syncMessage: "正在初始化", notifications: make(map[string]notificationState), startupWarn: startupWarning,
		portAvailable: localPortAvailable, setAutostart: autostart.Set,
	}
	supervisor.SetStatusHandler(controller.handleStatus)
	var startOnce sync.Once
	start := func() { startOnce.Do(func() { go controller.start() }) }
	cleanup := func() {
		cancel()
		supervisor.Close()
	}
	return controller, start, cleanup
}

func (c *Controller) start() {
	if c.startupWarn != "" {
		c.toast("warning", "配置已恢复", c.startupWarn)
		_ = c.platform.Notify("配置已恢复", c.startupWarn)
	}
	c.mu.RLock()
	settings := c.cfg.Settings
	rules := append([]model.ForwardRule(nil), c.cfg.Rules...)
	c.mu.RUnlock()
	if settings.StartFavoritesOnLaunch {
		if err := c.supervisor.StartFavorites(rules); err != nil {
			c.reportError(err)
		}
	}
	go c.repairAutostart(settings.LaunchAtLogin)
	c.Refresh()
	c.refreshLoop()
}

func (c *Controller) Bootstrap() AppSnapshot {
	return c.buildSnapshot(false)
}

func (c *Controller) Refresh() {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return
	}
	c.refreshing = true
	c.syncState = "syncing"
	c.syncMessage = "正在同步节点小宝服务..."
	credentialFile := c.cfg.Settings.CredentialFile
	c.mu.Unlock()
	c.provider.SetCredentialFile(credentialFile)
	c.publishSnapshot()
	go c.refresh()
}

func (c *Controller) refresh() {
	ctx, cancel := context.WithTimeout(c.ctx, 20*time.Second)
	defer cancel()
	type probeResult struct {
		runtime model.RuntimeInfo
		err     error
	}
	type serviceResult struct {
		services []model.RemoteService
		err      error
	}
	type wakeResult struct {
		targets []model.WakeTarget
		err     error
	}
	probeCh := make(chan probeResult, 1)
	serviceCh := make(chan serviceResult, 1)
	wakeCh := make(chan wakeResult, 1)
	go func() { value, err := c.provider.Probe(ctx); probeCh <- probeResult{value, err} }()
	go func() { value, err := c.provider.ListServices(ctx); serviceCh <- serviceResult{value, err} }()
	go func() { value, err := c.provider.ListWakeTargets(ctx); wakeCh <- wakeResult{value, err} }()

	probe, services, wake := <-probeCh, <-serviceCh, <-wakeCh
	c.mu.Lock()
	c.runtime = probe.runtime
	c.nodeErr = probe.err
	if wake.err == nil {
		c.wakeTargets = append([]model.WakeTarget(nil), wake.targets...)
	}
	c.mu.Unlock()
	if services.err == nil {
		c.mergeServices(services.services)
	}

	c.mu.Lock()
	c.refreshing = false
	c.lastSyncedAt = time.Now()
	switch {
	case services.err != nil:
		c.syncState = "error"
		c.syncMessage = "服务同步失败：" + services.err.Error()
	case probe.err != nil:
		c.syncState = "partial"
		c.syncMessage = "服务已同步；节点小宝本地服务不可用"
	case wake.err != nil:
		c.syncState = "partial"
		c.syncMessage = "服务已同步；唤醒设备读取失败：" + wake.err.Error()
	default:
		c.syncState = "ready"
		c.syncMessage = fmt.Sprintf("已同步 %d 个服务，%d 个可唤醒网卡", len(services.services), len(wake.targets))
	}
	c.mu.Unlock()
	c.publishSnapshot()
}

func (c *Controller) refreshLoop() {
	for {
		c.mu.RLock()
		minutes := c.cfg.Settings.RefreshMinutes
		c.mu.RUnlock()
		if minutes <= 0 {
			minutes = 5
		}
		timer := time.NewTimer(time.Duration(minutes) * time.Minute)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			c.Refresh()
		}
	}
}

func (c *Controller) mergeServices(services []model.RemoteService) {
	c.mu.Lock()
	changed := false
	byKey := make(map[string]int, len(c.cfg.Rules))
	usedPorts := make(map[int]struct{}, len(c.cfg.Rules))
	for index := range c.cfg.Rules {
		c.cfg.Rules[index].Available = false
		byKey[c.cfg.Rules[index].EndpointKey] = index
		usedPorts[c.cfg.Rules[index].ListenPort] = struct{}{}
	}
	for _, service := range services {
		if index, ok := byKey[service.EndpointKey]; ok {
			rule := &c.cfg.Rules[index]
			before := *rule
			rule.Name, rule.PeerID, rule.Host = service.Name, service.Endpoint.PeerID, service.Endpoint.Host
			rule.TargetPort, rule.Kind, rule.WebScheme = service.Endpoint.TargetPort, service.Kind, service.WebScheme
			rule.Icon, rule.Available = service.Icon, true
			changed = changed || !sameStoredRule(before, *rule)
			continue
		}
		port := suggestPort(service, usedPorts, c.portAvailable)
		usedPorts[port] = struct{}{}
		c.cfg.Rules = append(c.cfg.Rules, model.ForwardRule{
			EndpointKey: service.EndpointKey, Name: service.Name, PeerID: service.Endpoint.PeerID,
			Host: service.Endpoint.Host, TargetPort: service.Endpoint.TargetPort, ListenPort: port,
			Kind: service.Kind, WebScheme: service.WebScheme, Icon: service.Icon, Available: true,
		})
		changed = true
	}
	config.SortRules(c.cfg.Rules)
	cfg := c.configSnapshotLocked()
	c.mu.Unlock()
	if changed {
		if err := c.store.Save(cfg); err != nil && c.logger != nil {
			c.logger.Error("save merged services failed", "error", err)
		}
	}
	if err := c.supervisor.SyncRules(cfg.Rules); err != nil && c.logger != nil {
		c.logger.Warn("sync active rules failed", "error", err)
	}
}

func (c *Controller) handleStatus(status manager.Status) {
	c.mu.Lock()
	state := c.notifications[status.EndpointKey]
	title, message := "", ""
	switch status.State {
	case manager.StateReady:
		if state.failed {
			title, message = "转发已恢复", c.ruleNameLocked(status.EndpointKey)+" 已恢复连接"
		}
		state.hadReady, state.failed = true, false
	case manager.StateWaiting, manager.StateError:
		if state.hadReady && !state.failed {
			title, message = "转发连接异常", c.ruleNameLocked(status.EndpointKey)+"："+status.Message
			state.failed = true
		}
	case manager.StateDisabled:
		state = notificationState{}
	}
	c.notifications[status.EndpointKey] = state
	c.mu.Unlock()
	if title != "" {
		_ = c.platform.Notify(title, message)
	}
	c.publishSnapshot()
}

func (c *Controller) publishSnapshot() {
	c.platform.Emit(EventSnapshot, c.buildSnapshot(true))
}

func (c *Controller) toast(kind, title, message string) {
	c.platform.Emit(EventToast, ToastEvent{Kind: kind, Title: title, Message: message})
}

func (c *Controller) reportError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	c.toast("error", "操作失败", err.Error())
	_ = c.platform.Notify("节点小宝伴侣操作失败", err.Error())
}

func (c *Controller) buildSnapshot(increment bool) AppSnapshot {
	c.mu.Lock()
	if increment {
		c.revision++
	}
	revision := c.revision
	if revision == 0 {
		revision = 1
	}
	cfg := c.configSnapshotLocked()
	wakeTargets := append([]model.WakeTarget(nil), c.wakeTargets...)
	runtimeInfo, nodeErr := c.runtime, c.nodeErr
	syncState, syncMessage, lastSyncedAt := c.syncState, c.syncMessage, c.lastSyncedAt
	c.mu.Unlock()
	statuses := c.supervisor.Statuses()
	services := make([]ServiceSnapshot, 0, len(cfg.Rules))
	summary := AppSummary{Total: len(cfg.Rules)}
	for _, rule := range cfg.Rules {
		status := statuses[rule.EndpointKey]
		running := c.supervisor.Running(rule.EndpointKey)
		if rule.Favorite {
			summary.Favorites++
		}
		if running {
			summary.Running++
			summary.ActiveConnections += status.Active
		}
		if running && (status.State == manager.StateError || status.State == manager.StateWaiting) {
			summary.Errors++
		}
		services = append(services, ServiceSnapshot{
			EndpointKey: rule.EndpointKey, Name: rule.Name, Host: rule.Host, TargetPort: rule.TargetPort,
			TargetAddress: net.JoinHostPort(rule.Host, strconv.Itoa(rule.TargetPort)), ListenPort: rule.ListenPort,
			LocalAddress: rule.LocalAddress(), Kind: rule.Kind, WebScheme: model.NormalizeWebScheme(rule.WebScheme),
			Favorite: rule.Favorite, Available: rule.Available, Running: running, State: status.State,
			StateLabel: statusText(rule, status, running), Message: status.Message,
			ActiveConnections: status.Active, CanOpen: rule.Kind != model.ServiceKindTCP && status.State == manager.StateReady,
		})
	}
	node := NodeSnapshot{Message: "节点小宝本地服务未运行"}
	if nodeErr == nil && runtimeInfo.APIBase != "" {
		node = NodeSnapshot{Connected: true, Version: runtimeInfo.Version, APIBase: runtimeInfo.APIBase, Message: "本地服务已连接"}
	} else if nodeErr != nil {
		node.Message = nodeErr.Error()
	}
	wake := make([]WakeTargetSnapshot, 0, len(wakeTargets))
	for _, target := range wakeTargets {
		wake = append(wake, WakeTargetSnapshot{TargetKey: wakeTargetKey(target), Name: target.Name, MaskedMAC: maskMAC(target.MAC), Online: target.Online})
	}
	last := ""
	if !lastSyncedAt.IsZero() {
		last = lastSyncedAt.Format(time.RFC3339)
	}
	return AppSnapshot{Revision: revision, Version: c.version, SyncState: syncState, SyncMessage: syncMessage, LastSyncedAt: last, Node: node, Summary: summary, Services: services, WakeTargets: wake, Settings: settingsSnapshot(cfg.Settings)}
}

func (c *Controller) configSnapshotLocked() config.Config {
	cfg := c.cfg
	cfg.Rules = append([]model.ForwardRule(nil), c.cfg.Rules...)
	return cfg
}

func (c *Controller) ruleNameLocked(key string) string {
	for _, rule := range c.cfg.Rules {
		if rule.EndpointKey == key {
			return rule.Name
		}
	}
	return "固定端口"
}

func sameStoredRule(a, b model.ForwardRule) bool {
	return a.EndpointKey == b.EndpointKey && a.Name == b.Name && a.PeerID == b.PeerID && a.Host == b.Host &&
		a.TargetPort == b.TargetPort && a.ListenPort == b.ListenPort && a.Kind == b.Kind && a.Favorite == b.Favorite &&
		a.WebScheme == b.WebScheme && a.Icon == b.Icon
}

func statusText(rule model.ForwardRule, status manager.Status, running bool) string {
	if !running {
		if !rule.Available {
			return "服务不可用"
		}
		return "已停止"
	}
	switch status.State {
	case manager.StateReady:
		if status.Active > 0 {
			return fmt.Sprintf("已就绪 · %d 个连接", status.Active)
		}
		return "已就绪"
	case manager.StateMapping:
		return "正在创建映射"
	case manager.StateWaiting:
		return "等待节点小宝"
	case manager.StateError:
		return "转发错误"
	default:
		return "正在启动"
	}
}

func wakeTargetKey(target model.WakeTarget) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{target.PeerID, strings.ToLower(target.MAC)}, "|")))
	return hex.EncodeToString(sum[:])
}

func maskMAC(mac string) string {
	if len(mac) < 8 {
		return mac
	}
	return "**:**:**:" + mac[len(mac)-8:]
}

func suggestPort(service model.RemoteService, used map[int]struct{}, available func(int) bool) int {
	base := service.Endpoint.TargetPort + 10000
	switch service.Kind {
	case model.ServiceKindRDP:
		base = 13389
	case model.ServiceKindVNC:
		base = 15900
	case model.ServiceKindWeb:
		if service.WebScheme == "https" {
			base = 18443
		} else {
			base = 18080
		}
	}
	if base < 1024 || base > 65535 {
		base = 20000
	}
	for port := base; port <= 65535; port++ {
		if _, exists := used[port]; !exists && available(port) {
			return port
		}
	}
	for port := 1024; port < base; port++ {
		if _, exists := used[port]; !exists && available(port) {
			return port
		}
	}
	return base
}

func localPortAvailable(port int) bool {
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func (c *Controller) repairAutostart(enabled bool) {
	if !enabled {
		return
	}
	executable, err := os.Executable()
	if err == nil {
		err = c.setAutostart(true, executable)
	}
	if err != nil && c.logger != nil {
		c.logger.Warn("repair autostart failed", "error", err)
	}
}

func (c *Controller) diagnosticText() string {
	c.mu.RLock()
	cfg := c.configSnapshotLocked()
	runtimeInfo, nodeErr := c.runtime, c.nodeErr
	c.mu.RUnlock()
	statuses := c.supervisor.Statuses()
	lines := []string{"节点小宝固定端口伴侣 " + c.version, "系统: " + runtime.GOOS + "/" + runtime.GOARCH, "配置: " + c.store.Path()}
	if nodeErr != nil {
		lines = append(lines, "节点小宝: 不可用 ("+nodeErr.Error()+")")
	} else {
		lines = append(lines, "节点小宝: "+runtimeInfo.Version+" "+runtimeInfo.APIBase)
	}
	for _, rule := range cfg.Rules {
		status := statuses[rule.EndpointKey]
		lines = append(lines, fmt.Sprintf("规则: %s | local=%s | target=%s:%d | favorite=%t | running=%t | state=%s", rule.Name, rule.LocalAddress(), rule.Host, rule.TargetPort, rule.Favorite, c.supervisor.Running(rule.EndpointKey), status.State))
	}
	return strings.Join(lines, "\n")
}
