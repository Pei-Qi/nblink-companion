//go:build darwin

package autostart

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const launchAgentName = "com.local.nblink-companion.plist"
const launchAgentLabel = "com.local.nblink-companion"

func enable(executable string) error {
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("解析应用路径失败: %w", err)
	}
	if strings.Contains(executable, "/AppTranslocation/") {
		return errors.New("应用正在从 macOS 隔离的临时路径运行，请先将应用移到“应用程序”目录并正常打开一次")
	}
	if _, err := os.Stat(executable); err != nil {
		return fmt.Errorf("应用程序不存在: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, launchAgentName)
	arguments := fmt.Sprintf("<string>%s</string><string>--autostart</string>", xmlEscape(executable))
	if bundle := appBundlePath(executable); bundle != "" {
		arguments = fmt.Sprintf("<string>/usr/bin/open</string><string>-gj</string><string>%s</string>", xmlEscape(bundle))
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array>%s</array>
<key>RunAtLoad</key><true/>
<key>LimitLoadToSessionType</key><string>Aqua</string>
<key>ProcessType</key><string>Interactive</string>
</dict></plist>
`, launchAgentLabel, arguments)
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	service := domain + "/" + launchAgentLabel
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		if exec.Command("/bin/launchctl", "print", service).Run() == nil {
			return nil
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	_ = exec.Command("/bin/launchctl", "bootout", domain, path).Run()
	if output, err := exec.Command("/bin/launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("注册登录启动失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func isEnabled(executable string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(filepath.Join(home, "Library", "LaunchAgents", launchAgentName))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		resolved = executable
	}
	expected := resolved
	if bundle := appBundlePath(resolved); bundle != "" {
		expected = bundle
	}
	return strings.Contains(string(data), xmlEscape(expected)), nil
}

func appBundlePath(executable string) string {
	const marker = ".app/Contents/MacOS/"
	index := strings.Index(executable, marker)
	if index < 0 {
		return ""
	}
	return executable[:index+len(".app")]
}

func disable() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", launchAgentName)
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("/bin/launchctl", "bootout", domain, path).Run()
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func xmlEscape(value string) string {
	var data struct {
		Value string `xml:",chardata"`
	}
	data.Value = value
	encoded, _ := xml.Marshal(data)
	text := string(encoded)
	const prefix = "<struct>"
	const suffix = "</struct>"
	if len(text) >= len(prefix)+len(suffix) {
		return text[len(prefix) : len(text)-len(suffix)]
	}
	return value
}
