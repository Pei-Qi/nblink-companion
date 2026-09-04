package logx

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const maxLogSize = 5 << 20

func New() (*slog.Logger, io.Closer, error) {
	dir, err := Directory()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "companion.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= maxLogSize {
		_ = os.Remove(path + ".2")
		_ = os.Rename(path+".1", path+".2")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	writer := io.MultiWriter(os.Stderr, f)
	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})), f, nil
}

func Directory() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nblink-companion"), nil
}
