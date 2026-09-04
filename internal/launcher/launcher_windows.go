//go:build windows

package launcher

import "os/exec"

func openURL(value string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", value).Start()
}

func openPath(path string) error { return exec.Command("explorer.exe", path).Start() }

func openRDP(address, custom string) error {
	if custom != "" {
		return exec.Command(custom, address).Start()
	}
	return exec.Command("mstsc.exe", "/v:"+address).Start()
}

func openVNC(address, custom string) error {
	if custom == "" {
		return customLauncherMissing("VNC")
	}
	return exec.Command(custom, address).Start()
}
