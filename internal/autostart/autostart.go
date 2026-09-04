package autostart

func Set(enabled bool, executable string) error {
	if enabled {
		return enable(executable)
	}
	return disable()
}

func IsEnabled(executable string) (bool, error) {
	return isEnabled(executable)
}
