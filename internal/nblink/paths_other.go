//go:build !darwin && !windows

package nblink

import (
	"os"
	"path/filepath"
)

func platformCredentialCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".config", "nblink", "user_service.db")}
}

func platformLogCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".local", "share", "nblink", "nblink.log")}
}

func platformSearchRoots() []string { return nil }
