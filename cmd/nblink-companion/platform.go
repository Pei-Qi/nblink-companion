package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/local/nblink-companion/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

type desktopPlatform struct {
	mu         sync.RWMutex
	app        *application.App
	window     *application.WebviewWindow
	tray       *application.SystemTray
	controller *appservice.Controller
	notifier   *notifications.NotificationService
}

func (p *desktopPlatform) bind(app *application.App, window *application.WebviewWindow, tray *application.SystemTray, controller *appservice.Controller) {
	p.mu.Lock()
	p.app, p.window, p.tray, p.controller = app, window, tray, controller
	p.mu.Unlock()
}

func (p *desktopPlatform) Emit(name string, payload any) {
	p.mu.RLock()
	app := p.app
	p.mu.RUnlock()
	if app != nil {
		app.Event.Emit(name, payload)
	}
	if name == appservice.EventSnapshot {
		if snapshot, ok := payload.(appservice.AppSnapshot); ok {
			p.updateTray(snapshot)
		}
	}
}

func (p *desktopPlatform) Notify(title, message string) error {
	if p.notifier == nil {
		return errors.New("notification service is unavailable")
	}
	return p.notifier.SendNotification(notifications.NotificationOptions{
		ID: fmt.Sprintf("nblink-%d", time.Now().UnixNano()), Title: title, Body: message,
		ThreadID: "nblink-forwarding", InterruptionLevel: notifications.InterruptionLevelActive,
	})
}

func (p *desktopPlatform) SetClipboard(text string) error {
	p.mu.RLock()
	app := p.app
	p.mu.RUnlock()
	if app == nil || !app.Clipboard.SetText(text) {
		return errors.New("无法写入系统剪贴板")
	}
	return nil
}

func (p *desktopPlatform) ChooseFile(kind string) (string, error) {
	p.mu.RLock()
	app, window := p.app, p.window
	p.mu.RUnlock()
	if app == nil {
		return "", errors.New("文件选择器不可用")
	}
	dialog := app.Dialog.OpenFile().CanChooseFiles(true).CanChooseDirectories(false)
	if window != nil {
		dialog.AttachToWindow(window)
	}
	switch kind {
	case "credential":
		dialog.SetTitle("选择节点小宝数据文件").AddFilter("数据库文件", "*.db;*.sqlite;*.sqlite3")
	case "rdp":
		dialog.SetTitle("选择 RDP 客户端")
	case "vnc":
		dialog.SetTitle("选择 VNC 客户端")
	default:
		return "", errors.New("不支持的文件类型")
	}
	return dialog.PromptForSingleSelection()
}

func (p *desktopPlatform) updateTray(snapshot appservice.AppSnapshot) {
	p.mu.RLock()
	app, window, tray, controller := p.app, p.window, p.tray, p.controller
	p.mu.RUnlock()
	if app == nil || tray == nil || controller == nil {
		return
	}
	menu := app.NewMenu()
	menu.Add(fmt.Sprintf("运行 %d · 异常 %d · 连接 %d", snapshot.Summary.Running, snapshot.Summary.Errors, snapshot.Summary.ActiveConnections)).SetEnabled(false)
	menu.AddSeparator()
	for _, service := range snapshot.Services {
		if !service.Favorite {
			continue
		}
		current := service
		submenu := menu.AddSubmenu(current.Name)
		submenu.Add(current.StateLabel).SetEnabled(false)
		toggle := "启动"
		if current.Running {
			toggle = "停止"
		}
		submenu.Add(toggle).OnClick(func(*application.Context) { _ = controller.ToggleRule(current.EndpointKey) })
		submenu.Add("复制地址").OnClick(func(*application.Context) { _ = controller.CopyAddress(current.EndpointKey) })
		if current.Kind != "tcp" {
			submenu.Add("打开").SetEnabled(current.CanOpen).OnClick(func(*application.Context) { _ = controller.OpenRule(current.EndpointKey) })
		}
	}
	menu.AddSeparator()
	menu.Add("显示窗口").OnClick(func(*application.Context) { showWindow(window) })
	menu.Add("停止全部转发").SetEnabled(snapshot.Summary.Running > 0).OnClick(func(*application.Context) { controller.StopAll() })
	menu.Add("刷新服务").OnClick(func(*application.Context) { controller.Refresh() })
	menu.Add("设置").OnClick(func(*application.Context) {
		showWindow(window)
		app.Event.Emit(appservice.EventNavigate, "settings")
	})
	menu.AddSeparator()
	menu.Add("打开日志目录").OnClick(func(*application.Context) { _ = controller.OpenLogs() })
	menu.Add("复制诊断信息").OnClick(func(*application.Context) { _ = controller.CopyDiagnostics() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.SetTooltip(fmt.Sprintf("Nblink Companion · 运行 %d · 异常 %d", snapshot.Summary.Running, snapshot.Summary.Errors))
}
