// Package schedule owns the OS-level cadence: a launchd LaunchAgent that runs the
// binary headless, and the runner that decides which folders are due and backs
// them up. restic is not a daemon, so dotbackup owns the schedule.
package schedule

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jezweb/dotbackup/internal/backup"
	"github.com/jezweb/dotbackup/internal/config"
	"github.com/jezweb/dotbackup/internal/restic"
)

var retention = restic.RetentionPolicy{KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, Prune: true}

// RunScheduled backs up every folder marked backup:true whose last backup is
// older than the configured interval, applies retention, and records the run
// time. Invoked by launchd via `<binary> --run-scheduled`. No secret is read
// here: NewRunner pulls them from the keychain.
func RunScheduled(ctx context.Context, passwordCmd string) error {
	log.Printf("scheduled run start")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	runner, err := backup.NewRunner(cfg, passwordCmd)
	if err != nil {
		return err
	}
	now := time.Now()
	interval := time.Duration(cfg.Schedule.EveryHours) * time.Hour

	var changed bool
	for i := range cfg.Folders {
		f := &cfg.Folders[i]
		if !f.Backup || !due(f.LastBackupAt, now, interval) {
			continue
		}
		excludeFile, cleanup, err := writeExcludeFile(f.Excludes)
		if err != nil {
			return err
		}
		log.Printf("backing up %s", f.Path)
		summary, err := runner.Backup(ctx, []string{f.Path}, excludeFile, nil)
		cleanup()
		if err != nil {
			return fmt.Errorf("backup %s: %w", f.Path, err)
		}
		log.Printf("backed up %s -> snapshot %s", f.Path, shortID(summary.SnapshotID))
		f.LastBackupAt = now.Format(time.RFC3339) // offset-bearing, stamped by the runtime
		changed = true
	}

	if changed {
		if err := runner.Forget(ctx, retention); err != nil {
			return fmt.Errorf("retention: %w", err)
		}
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}
	log.Printf("scheduled run done (changed=%v)", changed)
	return nil
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func due(last string, now time.Time, interval time.Duration) bool {
	if last == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return true
	}
	return now.Sub(t) >= interval
}

// writeExcludeFile materialises restic's --exclude-file; restic reads patterns
// from a file so they never crowd argv. Returns a cleanup closer.
func writeExcludeFile(excludes []string) (string, func(), error) {
	if len(excludes) == 0 {
		return "", func() {}, nil
	}
	f, err := os.CreateTemp("", "dotbackup-excludes-*.txt")
	if err != nil {
		return "", func() {}, err
	}
	for _, e := range excludes {
		fmt.Fprintln(f, e)
	}
	_ = f.Close()
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}
