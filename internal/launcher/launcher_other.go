//go:build !darwin && !windows

package launcher

import "os/exec"

func openURL(value string) error { return exec.Command("xdg-open", value).Start() }
func openPath(path string) error { return exec.Command("xdg-open", path).Start() }

func openRDP(address, custom string) error {
	if custom == "" {
		return customLauncherMissing("RDP")
	}
	return exec.Command(custom, address).Start()
}

func openVNC(address, custom string) error {
	if custom == "" {
		return customLauncherMissing("VNC")
	}
	return exec.Command(custom, address).Start()
}
