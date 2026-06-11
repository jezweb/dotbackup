package config

import (
	"os"
	"path/filepath"
)

// AppSupportDir is ~/Library/Application Support/dotbackup on macOS.
func AppSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "dotbackup"), nil
}

func ConfigPath() (string, error) {
	dir, err := AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
