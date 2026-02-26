package xedir

import (
	"os"
	"path/filepath"
	"runtime"
)

func Home() (string, error) {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "xe"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Local", "xe"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "xe"), nil
}

func MustHome() string {
	home, err := Home()
	if err != nil {
		return "xe"
	}
	return home
}

func ConfigFile() string {
	return filepath.Join(MustHome(), "config.yaml")
}

func CacheDir() string {
	return filepath.Join(MustHome(), "cache")
}

func VenvDir() string {
	return filepath.Join(MustHome(), "venvs")
}

func ShimDir() string {
	return filepath.Join(MustHome(), "bin")
}

func PluginDir() string {
	return filepath.Join(MustHome(), "plugins")
}

func EnsureHome() error {
	return os.MkdirAll(MustHome(), 0755)
}
