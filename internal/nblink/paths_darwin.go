//go:build darwin

package nblink

import (
	"os"
	"path/filepath"
)

func platformCredentialCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, "Library", "Containers", "com.ionewu.macos.jdxb", "Data", "Documents", "nblink", "user_service.db"),
	}
}

func platformLogCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, "Library", "Containers", "com.ionewu.macos.jdxb", "Data", "Documents", "nblink", "nblink.log"),
	}
}

func platformSearchRoots() []string { return nil }
