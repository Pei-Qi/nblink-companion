package appservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/launcher"
	"github.com/local/nblink-companion/internal/logx"
	"github.com/local/nblink-companion/internal/model"
)

func (c *Controller) SetFavorite(endpointKey string, favorite bool) error {
	c.mu.Lock()
	index := c.ruleIndexLocked(endpointKey)
	if index < 0 {
		c.mu.Unlock()
		return errors.New("服务不存在或已被移除")
	}
	if c.cfg.Rules[index].Favorite == favorite {
		c.mu.Unlock()
		return nil
	}
	c.cfg.Rules[index].Favorite = favorite
	config.SortRules(c.cfg.Rules)
	cfg := c.configSnapshotLocked()
	c.mu.Unlock()
	if err := c.store.Save(cfg); err != nil {
		return err
	}
	c.publishSnapshot()
	return nil
}

func (c *Controller) ToggleRule(endpointKey string) error {
	if c.supervisor.Running(endpointKey) {
		c.supervisor.Stop(endpointKey)
		c.publishSnapshot()
		return nil
	}
	rule, ok := c.ruleByKey(endpointKey)
	if !ok {
		return errors.New("服务不存在或已被移除")
	}
	if err := c.supervisor.Start(rule); err != nil {
		c.reportError(err)
		return err
	}
	c.publishSnapshot()
	return nil
}

func (c *Controller) StopAll() {
	c.supervisor.StopAll()
	c.toast("info", "全部转发已停止", "常用设置未更改")
	c.publishSnapshot()
}

func (c *Controller) UpdateRule(endpointKey string, patch RulePatch) error {
	if patch.ListenPort < 1024 || patch.ListenPort > 65535 {
		return errors.New("固定端口必须在 1024～65535 之间")
	}
	switch patch.Kind {
	case model.ServiceKindTCP, model.ServiceKindWeb, model.ServiceKindRDP, model.ServiceKindVNC:
	default:
		return errors.New("不支持的服务类型")
	}
	c.mu.Lock()
	index := c.ruleIndexLocked(endpointKey)
	if index < 0 {
		c.mu.Unlock()
		return errors.New("服务不存在或已被移除")
	}
	current := c.cfg.Rules[index]
	for _, other := range c.cfg.Rules {
		if other.EndpointKey != endpointKey && other.ListenPort == patch.ListenPort {
			c.mu.Unlock()
			return fmt.Errorf("端口 %d 已由 %s 使用", patch.ListenPort, other.Name)
		}
	}
	if patch.ListenPort != current.ListenPort && !c.portAvailable(patch.ListenPort) {
		c.mu.Unlock()
		return fmt.Errorf("端口 %d 已被其他程序占用", patch.ListenPort)
	}
	c.cfg.Rules[index].ListenPort = patch.ListenPort
	c.cfg.Rules[index].Kind = patch.Kind
	c.cfg.Rules[index].WebScheme = model.NormalizeWebScheme(patch.WebScheme)
	cfg := c.configSnapshotLocked()
	c.mu.Unlock()
	if err := c.store.Save(cfg); err != nil {
		return err
	}
	if err := c.supervisor.SyncRules(cfg.Rules); err != nil {
		return err
	}
	c.publishSnapshot()
	return nil
}

func (c *Controller) OpenRule(endpointKey string) error {
	rule, ok := c.ruleByKey(endpointKey)
	if !ok {
		return errors.New("服务不存在或已被移除")
	}
	c.mu.RLock()
	settings := c.cfg.Settings
	c.mu.RUnlock()
	return launcher.Open(rule, settings)
}

func (c *Controller) CopyAddress(endpointKey string) error {
	rule, ok := c.ruleByKey(endpointKey)
	if !ok {
		return errors.New("服务不存在或已被移除")
	}
	if err := c.platform.SetClipboard(rule.LocalAddress()); err != nil {
		return err
	}
	c.toast("success", "已复制", rule.LocalAddress())
	return nil
}

func (c *Controller) Wake(targetKey string) error {
	c.mu.RLock()
	targets := append([]model.WakeTarget(nil), c.wakeTargets...)
	c.mu.RUnlock()
	var target model.WakeTarget
	found := false
	for _, candidate := range targets {
		if wakeTargetKey(candidate) == targetKey {
			target, found = candidate, true
			break
		}
	}
	if !found {
		return errors.New("唤醒设备不存在或已失效，请刷新后重试")
	}
	ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	if err := c.provider.Wake(ctx, target); err != nil {
		return err
	}
	c.toast("success", "已发送唤醒请求", target.Name)
	return nil
}

func (c *Controller) SaveSettings(input SettingsInput) error {
	if input.RefreshMinutes <= 0 || input.RefreshMinutes > 1440 {
		return errors.New("刷新周期必须是 1～1440 分钟")
	}
	theme := config.NormalizeThemeMode(input.ThemeMode)
	if input.ThemeMode != "" && theme != input.ThemeMode {
		return errors.New("主题设置无效")
	}
	settings := config.Settings{
		LaunchAtLogin: input.LaunchAtLogin, StartFavoritesOnLaunch: input.StartFavoritesOnLaunch,
		CredentialFile: strings.TrimSpace(input.CredentialFile), RDPLauncher: strings.TrimSpace(input.RDPLauncher),
		VNCLauncher: strings.TrimSpace(input.VNCLauncher), RefreshMinutes: input.RefreshMinutes, ThemeMode: theme,
	}
	c.mu.RLock()
	previous := c.cfg.Settings
	c.mu.RUnlock()
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err := c.setAutostart(settings.LaunchAtLogin, executable); err != nil {
		return err
	}
	c.mu.Lock()
	c.cfg.Settings = settings
	cfg := c.configSnapshotLocked()
	c.mu.Unlock()
	if err := c.store.Save(cfg); err != nil {
		_ = c.setAutostart(previous.LaunchAtLogin, executable)
		c.mu.Lock()
		c.cfg.Settings = previous
		c.mu.Unlock()
		return err
	}
	c.provider.SetCredentialFile(settings.CredentialFile)
	c.toast("success", "设置已保存", "新的设置已立即生效")
	c.publishSnapshot()
	c.Refresh()
	return nil
}

func (c *Controller) ChooseFile(kind string) (string, error) {
	switch kind {
	case "credential", "rdp", "vnc":
	default:
		return "", errors.New("不支持的文件类型")
	}
	return c.platform.ChooseFile(kind)
}

func (c *Controller) OpenLogs() error {
	dir, err := logx.Directory()
	if err != nil {
		return err
	}
	return launcher.OpenPath(dir)
}

func (c *Controller) CopyDiagnostics() error {
	if err := c.platform.SetClipboard(c.diagnosticText()); err != nil {
		return err
	}
	c.toast("success", "诊断信息已复制", "内容不包含节点小宝凭据")
	return nil
}

func (c *Controller) ruleIndexLocked(key string) int {
	for index := range c.cfg.Rules {
		if c.cfg.Rules[index].EndpointKey == key {
			return index
		}
	}
	return -1
}

func (c *Controller) ruleByKey(key string) (model.ForwardRule, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rule := range c.cfg.Rules {
		if rule.EndpointKey == key {
			return rule, true
		}
	}
	return model.ForwardRule{}, false
}
