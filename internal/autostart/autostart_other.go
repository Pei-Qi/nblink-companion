//go:build !darwin && !windows

package autostart

import "errors"

func enable(string) error            { return errors.New("当前平台不支持开机启动") }
func disable() error                 { return nil }
func isEnabled(string) (bool, error) { return false, nil }
