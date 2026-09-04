//go:build darwin

package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func openURL(value string) error {
	return exec.Command("open", value).Start()
}

func openPath(path string) error { return exec.Command("open", path).Start() }

func openRDP(address, custom string) error {
	if custom != "" {
		return startCustom(custom, "rdp://"+address)
	}
	f, err := os.CreateTemp("", "nblink-*.rdp")
	if err != nil {
		return err
	}
	path := f.Name()
	if _, err := fmt.Fprintf(f, "full address:s:%s\nprompt for credentials:i:1\n", address); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return exec.Command("open", path).Start()
}

func openVNC(address, custom string) error {
	if custom != "" {
		return startCustom(custom, "vnc://"+address)
	}
	return openURL("vnc://" + address)
}

func startCustom(custom, address string) error {
	if strings.HasSuffix(strings.ToLower(custom), ".app") || filepath.Ext(custom) == "" {
		return exec.Command("open", "-a", custom, address).Start()
	}
	return exec.Command(custom, address).Start()
}
