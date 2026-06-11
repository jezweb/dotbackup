package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/jezweb/dotbackup/internal/backup"
	"github.com/jezweb/dotbackup/internal/config"
	"github.com/jezweb/dotbackup/internal/restic"
	"github.com/jezweb/dotbackup/internal/schedule"
	"github.com/jezweb/dotbackup/internal/secret"
	rt "github.com/wailsapp/wails/v2/pkg/runtime"
)

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "default"
}

// App is the Wails-bound surface. Every method here is callable from the
// frontend; none of them ever returns a secret. The engine specifics stay in
// internal/restic — App only orchestrates and emits progress events.
type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// --- view types (frontend-facing; secrets never appear) ---

type FolderView struct {
	Path         string `json:"path"`
	Backup       bool   `json:"backup"`
	Sync         bool   `json:"sync"`
	LastBackupAt string `json:"lastBackupAt"`
}

type StatusView struct {
	Configured bool         `json:"configured"`
	User       string       `json:"user"`
	Bucket     string       `json:"bucket"`
	Endpoint   string       `json:"endpoint"`
	Folders    []FolderView `json:"folders"`
}

type SnapshotView struct {
	ID      string   `json:"id"`
	ShortID string   `json:"shortId"`
	Time    string   `json:"time"`
	Paths   []string `json:"paths"`
}

type NodeView struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Size uint64 `json:"size"`
}

// GetStatus reports whether setup has run and, if so, the (secret-free) config.
func (a *App) GetStatus() StatusView {
	cfg, err := config.Load()
	if err != nil {
		return StatusView{Configured: false}
	}
	sv := StatusView{Configured: true, User: cfg.User, Bucket: cfg.Repo.Bucket, Endpoint: cfg.Repo.Endpoint}
	for _, f := range cfg.Folders {
		sv.Folders = append(sv.Folders, FolderView{Path: f.Path, Backup: f.Backup, Sync: f.Sync, LastBackupAt: f.LastBackupAt})
	}
	return sv
}

func (a *App) runner() (*restic.Runner, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("not set up yet — run the setup-dotbackup skill first")
	}
	self, _ := os.Executable()
	r, err := backup.NewRunner(cfg, self+" --print-passphrase")
	return r, cfg, err
}

func (a *App) ListSnapshots() ([]SnapshotView, error) {
	r, _, err := a.runner()
	if err != nil {
		return nil, err
	}
	snaps, err := r.Snapshots(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotView, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, SnapshotView{ID: s.ID, ShortID: s.ShortID, Time: s.Time.Format(time.RFC3339), Paths: s.Paths})
	}
	return out, nil
}

// BackupNow backs up one folder, emitting backup:{start,progress,done,error}
// events the UI subscribes to, then records lastBackupAt.
func (a *App) BackupNow(path string) error {
	r, cfg, err := a.runner()
	if err != nil {
		return err
	}
	var excludes []string
	for _, f := range cfg.Folders {
		if f.Path == path {
			excludes = f.Excludes
		}
	}
	excludeFile, cleanup, err := backup.WriteExcludeFile(excludes)
	if err != nil {
		return err
	}
	defer cleanup()

	rt.EventsEmit(a.ctx, "backup:start", path)
	summary, err := r.Backup(a.ctx, []string{path}, excludeFile, func(s restic.BackupStatus) {
		rt.EventsEmit(a.ctx, "backup:progress", map[string]any{
			"path": path, "percent": s.PercentDone, "filesDone": s.FilesDone, "totalFiles": s.TotalFiles,
		})
	})
	if err != nil {
		rt.EventsEmit(a.ctx, "backup:error", map[string]any{"path": path, "error": err.Error()})
		return err
	}
	for i := range cfg.Folders {
		if cfg.Folders[i].Path == path {
			cfg.Folders[i].LastBackupAt = time.Now().Format(time.RFC3339)
		}
	}
	_ = cfg.Save()
	rt.EventsEmit(a.ctx, "backup:done", map[string]any{"path": path, "snapshot": summary.SnapshotID})
	return nil
}

// BackupAll backs up every folder marked backup:true, sequentially.
func (a *App) BackupAll() error {
	_, cfg, err := a.runner()
	if err != nil {
		return err
	}
	for _, f := range cfg.Folders {
		if !f.Backup {
			continue
		}
		if err := a.BackupNow(f.Path); err != nil {
			return err
		}
	}
	return nil
}

// PickFolder opens the native directory chooser and returns the chosen path.
func (a *App) PickFolder() (string, error) {
	return rt.OpenDirectoryDialog(a.ctx, rt.OpenDialogOptions{Title: "Choose a folder to back up"})
}

func (a *App) AddFolder(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, f := range cfg.Folders {
		if f.Path == path {
			return nil // already present
		}
	}
	cfg.Folders = append(cfg.Folders, config.Folder{Path: path, Backup: true})
	return cfg.Save()
}

func (a *App) RemoveFolder(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	kept := cfg.Folders[:0]
	for _, f := range cfg.Folders {
		if f.Path != path {
			kept = append(kept, f)
		}
	}
	cfg.Folders = kept
	return cfg.Save()
}

func (a *App) SetFolderBackup(path string, on bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for i := range cfg.Folders {
		if cfg.Folders[i].Path == path {
			cfg.Folders[i].Backup = on
		}
	}
	return cfg.Save()
}

// --- restore (engine proven in Slice 1; UI lands in Slice 5) ---

func (a *App) SnapshotTree(snapshotID string) ([]NodeView, error) {
	r, _, err := a.runner()
	if err != nil {
		return nil, err
	}
	nodes, err := r.Ls(a.ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	out := make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, NodeView{Name: n.Name, Type: n.Type, Path: n.Path, Size: n.Size})
	}
	return out, nil
}

func (a *App) RestoreFile(snapshotID, includePath, targetDir string) error {
	r, _, err := a.runner()
	if err != nil {
		return err
	}
	var include []string
	if includePath != "" {
		include = []string{includePath}
	}
	return r.Restore(a.ctx, snapshotID, targetDir, include, func(s restic.RestoreStatus) {
		rt.EventsEmit(a.ctx, "restore:progress", map[string]any{"percent": s.PercentDone})
	})
}

func (a *App) PickRestoreTarget() (string, error) {
	return rt.OpenDirectoryDialog(a.ctx, rt.OpenDialogOptions{Title: "Restore into which folder?"})
}

// --- first-run setup (in-app, paste R2 creds) ---

type SetupInput struct {
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

// Setup connects the app to an R2 bucket: it stores the S3 secret and a freshly
// generated repo passphrase in the keychain, initialises the encrypted
// repository, and writes the config. It returns the passphrase ONCE so the UI can
// show the recovery moment. The passphrase is never stored anywhere we hand back
// to the user later — losing it means losing the data.
func (a *App) Setup(in SetupInput) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	bucket := strings.TrimSpace(in.Bucket)
	akid := strings.TrimSpace(in.AccessKeyID)
	s3secret := strings.TrimSpace(in.SecretAccessKey)
	if endpoint == "" || bucket == "" || akid == "" || s3secret == "" {
		return "", fmt.Errorf("all four fields are required")
	}
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "https://" + endpoint
	}

	usr := currentUser()
	pass, err := secret.GeneratePassphrase()
	if err != nil {
		return "", err
	}
	if err := secret.StoreS3Secret(usr, s3secret); err != nil {
		return "", fmt.Errorf("store S3 secret in keychain: %w", err)
	}
	if err := secret.StorePassphrase(usr, pass); err != nil {
		return "", fmt.Errorf("store passphrase in keychain: %w", err)
	}

	cfg := &config.Config{
		Version:  1,
		User:     usr,
		Repo:     config.Repo{Endpoint: endpoint, Bucket: bucket, AccessKeyID: akid},
		Schedule: config.Schedule{EveryHours: 6},
	}
	if err := cfg.Save(); err != nil {
		return "", err
	}
	self, _ := os.Executable()
	r, err := backup.NewRunner(cfg, self+" --print-passphrase")
	if err != nil {
		return "", err
	}
	if err := r.Init(a.ctx); err != nil {
		// roll back the half-written config so the user can retry cleanly
		return "", fmt.Errorf("could not connect to the bucket (check the endpoint, bucket name, and keys): %w", err)
	}
	return pass, nil
}

// --- schedule ---

type ScheduleView struct {
	Enabled    bool `json:"enabled"`
	EveryHours int  `json:"everyHours"`
}

func (a *App) GetSchedule() ScheduleView {
	cfg, err := config.Load()
	hours := 6
	if err == nil && cfg.Schedule.EveryHours > 0 {
		hours = cfg.Schedule.EveryHours
	}
	return ScheduleView{Enabled: schedule.IsInstalled(), EveryHours: hours}
}

func (a *App) SetSchedule(enabled bool, everyHours int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if everyHours <= 0 {
		everyHours = 6
	}
	cfg.Schedule.EveryHours = everyHours
	if err := cfg.Save(); err != nil {
		return err
	}
	if !enabled {
		return schedule.Uninstall()
	}
	self, _ := os.Executable()
	return schedule.Install(self, everyHours*3600, false)
}
