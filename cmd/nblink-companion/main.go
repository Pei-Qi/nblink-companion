package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/local/nblink-companion/assets"
	"github.com/local/nblink-companion/frontend"
	"github.com/local/nblink-companion/internal/appservice"
	"github.com/local/nblink-companion/internal/config"
	"github.com/local/nblink-companion/internal/logx"
	"github.com/local/nblink-companion/internal/manager"
	"github.com/local/nblink-companion/internal/nblink"
	"github.com/local/nblink-companion/internal/singleinstance"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const version = "0.3.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	lock, err := singleinstance.Acquire()
	if err != nil {
		if errors.Is(err, singleinstance.ErrActivated) {
			return
		}
		fmt.Fprintln(os.Stderr, "节点小宝固定端口伴侣已在运行")
		os.Exit(1)
	}
	defer lock.Close()

	logger, closer, err := logx.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closer.Close()

	store, cfg, startupWarning, err := loadConfig(logger)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	provider := nblink.NewClient(logger, nblink.WithCredentialFile(cfg.Settings.CredentialFile))
	supervisor := manager.New(provider, logger)
	notifier := notifications.New()
	platform := &desktopPlatform{notifier: notifier}
	controller, startController, stopController := appservice.New(
		logger, store, provider, supervisor, platform, cfg, version, startupWarning,
	)

	app := application.New(application.Options{
		Name:        "Nblink Companion",
		Description: "节点小宝固定端口伴侣",
		Icon:        assets.AppIconPNG,
		Logger:      logger,
		Services: []application.Service{
			application.NewService(controller),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{
			Handler:        application.BundledAssetFileServer(frontend.Dist()),
			DisableLogging: true,
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows:    application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		OnShutdown: stopController,
		ErrorHandler: func(err error) {
			logger.Error("wails error", "error", err)
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "main",
		Title:                      "节点小宝固定端口伴侣",
		Width:                      1080,
		Height:                     700,
		MinWidth:                   900,
		MinHeight:                  600,
		Hidden:                     true,
		URL:                        "/",
		BackgroundColour:           application.NewRGB(245, 246, 247),
		DefaultContextMenuDisabled: true,
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("节点小宝固定端口伴侣")
	tray.SetIcon(assets.TrayIconPNG)
	tray.OnClick(func() { showWindow(window) })
	platform.bind(app, window, tray, controller)
	platform.updateTray(controller.Bootstrap())

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		_, _ = notifier.RequestNotificationAuthorization()
		startController()
	})
	go func() {
		for range lock.Activations() {
			showWindow(window)
		}
	}()

	if err := app.Run(); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func loadConfig(logger interface {
	Error(string, ...any)
	Warn(string, ...any)
}) (*config.Store, config.Config, string, error) {
	configPath, err := config.DefaultPath()
	if err != nil {
		return nil, config.Config{}, "", err
	}
	store := config.NewStore(configPath)
	cfg, err := store.Load()
	if err == nil {
		if saveErr := store.Save(cfg); saveErr != nil {
			logger.Error("persist migrated config failed", "error", saveErr)
		}
		return store, cfg, "", nil
	}
	backup, backupErr := store.BackupInvalid()
	if backupErr != nil {
		return nil, config.Config{}, "", errors.Join(err, backupErr)
	}
	cfg = config.Default()
	if saveErr := store.Save(cfg); saveErr != nil {
		return nil, config.Config{}, "", saveErr
	}
	warning := "原配置无法读取，已备份到 " + backup
	logger.Warn("recovered invalid config", "error", err, "backup", backup)
	return store, cfg, warning, nil
}

func showWindow(window *application.WebviewWindow) {
	window.Show()
	window.Restore()
	window.Focus()
}
