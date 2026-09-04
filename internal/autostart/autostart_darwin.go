//go:build darwin

package autostart

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const launchAgentName = "com.local.nblink-companion.plist"

func enable(executable string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, launchAgentName)
	arguments := fmt.Sprintf("<string>%s</string>", xmlEscape(executable))
	if bundle := appBundlePath(executable); bundle != "" {
		arguments = fmt.Sprintf("<string>/usr/bin/open</string><string>-gj</string><string>%s</string>", xmlEscape(bundle))
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.local.nblink-companion</string>
<key>ProgramArguments</key><array>%s</array>
<key>RunAtLoad</key><true/>
</dict></plist>
`, arguments)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
	expected := executable
	if bundle := appBundlePath(executable); bundle != "" {
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
	err = os.Remove(filepath.Join(home, "Library", "LaunchAgents", launchAgentName))
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
