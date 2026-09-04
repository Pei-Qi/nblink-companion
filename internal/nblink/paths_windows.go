//go:build windows

package nblink

import (
	"os"
	"path/filepath"
)

func platformCredentialCandidates() []string {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	profile := os.Getenv("USERPROFILE")
	return []string{
		filepath.Join(local, "nblink", "user_service.db"),
		filepath.Join(local, "节点小宝", "user_service.db"),
		filepath.Join(roaming, "nblink", "user_service.db"),
		filepath.Join(profile, "Documents", "nblink", "user_service.db"),
	}
}

func platformLogCandidates() []string {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	return []string{
		filepath.Join(local, "nblink", "nblink.log"),
		filepath.Join(roaming, "nblink", "nblink.log"),
	}
}

func platformSearchRoots() []string {
	return []string{os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA")}
}
