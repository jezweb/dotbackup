package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Repo struct {
	Endpoint    string `json:"endpoint"`
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix,omitempty"`
	AccessKeyID string `json:"accessKeyId"`
}

type Folder struct {
	Path     string   `json:"path"`
	Backup   bool     `json:"backup"`
	Sync     bool     `json:"sync"`
	Excludes []string `json:"excludes,omitempty"`
	// LastBackupAt is an offset-bearing ISO 8601 instant, written by the Go
	// runtime after a successful backup — never typed by a model.
	LastBackupAt string `json:"lastBackupAt,omitempty"`
}

type Schedule struct {
	EveryHours int  `json:"everyHours"`
	OnlyOnWifi bool `json:"onlyOnWifi"`
}

type Config struct {
	Version  int      `json:"version"`
	User     string   `json:"user"`
	Repo     Repo     `json:"repo"`
	Folders  []Folder `json:"folders"`
	Schedule Schedule `json:"schedule"`
}

func Load() (*Config, error) {
	p, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Save() error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
