//go:build windows

package autostart

import (
	"bytes"
	"os/exec"
	"strconv"
)

const registryRunKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func enable(executable string) error {
	return exec.Command("reg", "add", registryRunKey, "/v", "NblinkCompanion", "/t", "REG_SZ", "/d", strconv.Quote(executable), "/f").Run()
}

func disable() error {
	cmd := exec.Command("reg", "delete", registryRunKey, "/v", "NblinkCompanion", "/f")
	if err := cmd.Run(); err != nil {
		// 值不存在与已经关闭开机启动等价。
		return nil
	}
	return nil
}

func isEnabled(executable string) (bool, error) {
	output, err := exec.Command("reg", "query", registryRunKey, "/v", "NblinkCompanion").Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return bytes.Contains(output, []byte(executable)), nil
}
