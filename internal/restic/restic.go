// Package restic drives a vendored restic binary over its --json contract.
// It is the engine-agnostic boundary of dotbackup: everything restic-specific
// lives here, the rest of the app talks only to Runner.
package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	Bin string   // absolute path to the restic binary
	Env []string // built by BuildEnv
}

// RetentionPolicy maps to restic forget --keep-* flags.
type RetentionPolicy struct {
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	Prune       bool
}

const scanBuf = 8 * 1024 * 1024

// lockRetry lets concurrent repo operations wait for a lock rather than failing —
// the app listing snapshots must not break a scheduled backup's forget, and vice
// versa. restic accepts --retry-lock on every lock-taking command (not init).
var lockRetry = []string{"--retry-lock", "2m"}

func (r *Runner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Env = r.Env
	return cmd
}

// run executes restic and buffers stdout; stderr is folded into the error.
func (r *Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := r.command(ctx, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return out.Bytes(), fmt.Errorf("restic %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func (r *Runner) Init(ctx context.Context) error {
	_, err := r.run(ctx, "init")
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already initialized") || strings.Contains(msg, "already exists") {
			return nil
		}
	}
	return err
}

// Backup streams `restic backup --json`, calling onProgress on each status line
// and returning the final summary.
func (r *Runner) Backup(ctx context.Context, paths []string, excludeFile string, onProgress func(BackupStatus)) (BackupSummary, error) {
	args := append([]string{"backup", "--json"}, lockRetry...)
	if excludeFile != "" {
		args = append(args, "--exclude-file", excludeFile)
	}
	args = append(args, paths...)

	cmd := r.command(ctx, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return BackupSummary{}, err
	}
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return BackupSummary{}, err
	}

	var summary BackupSummary
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), scanBuf)
	for sc.Scan() {
		line := sc.Bytes()
		switch peekType(line) {
		case "status":
			if onProgress != nil {
				var s BackupStatus
				if json.Unmarshal(line, &s) == nil {
					onProgress(s)
				}
			}
		case "summary":
			_ = json.Unmarshal(line, &summary)
		}
	}
	if err := cmd.Wait(); err != nil {
		return summary, fmt.Errorf("restic backup: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return summary, nil
}

func (r *Runner) Snapshots(ctx context.Context) ([]Snapshot, error) {
	out, err := r.run(ctx, append([]string{"snapshots", "--json"}, lockRetry...)...)
	if err != nil {
		return nil, err
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, fmt.Errorf("parse snapshots: %w", err)
	}
	return snaps, nil
}

// Ls returns the file/dir nodes of a snapshot. `restic ls --json` emits a header
// line then one line per node; we accept either the older struct_type or newer
// message_type tagging, and fall back to shape detection.
func (r *Runner) Ls(ctx context.Context, snapshotID string) ([]Node, error) {
	out, err := r.run(ctx, append([]string{"ls", snapshotID, "--json"}, lockRetry...)...)
	if err != nil {
		return nil, err
	}
	var nodes []Node
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), scanBuf)
	for sc.Scan() {
		line := sc.Bytes()
		var probe struct {
			StructType  string `json:"struct_type"`
			MessageType string `json:"message_type"`
			Name        string `json:"name"`
			Type        string `json:"type"`
		}
		if json.Unmarshal(line, &probe) != nil {
			continue
		}
		isNode := probe.StructType == "node" || probe.MessageType == "node" ||
			(probe.Name != "" && (probe.Type == "file" || probe.Type == "dir"))
		if !isNode {
			continue
		}
		var n Node
		if json.Unmarshal(line, &n) == nil {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// Restore extracts a snapshot (optionally filtered by include patterns) into
// target. restic recreates the original absolute path tree under target.
func (r *Runner) Restore(ctx context.Context, snapshotID, target string, include []string, onProgress func(RestoreStatus)) error {
	args := append([]string{"restore", snapshotID, "--target", target, "--json"}, lockRetry...)
	for _, inc := range include {
		args = append(args, "--include", inc)
	}
	cmd := r.command(ctx, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), scanBuf)
	for sc.Scan() {
		line := sc.Bytes()
		if onProgress != nil && peekType(line) == "status" {
			var s RestoreStatus
			if json.Unmarshal(line, &s) == nil {
				onProgress(s)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("restic restore: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}

func (r *Runner) Forget(ctx context.Context, policy RetentionPolicy) error {
	args := append([]string{"forget", "--json"}, lockRetry...)
	if policy.KeepDaily > 0 {
		args = append(args, "--keep-daily", fmt.Sprint(policy.KeepDaily))
	}
	if policy.KeepWeekly > 0 {
		args = append(args, "--keep-weekly", fmt.Sprint(policy.KeepWeekly))
	}
	if policy.KeepMonthly > 0 {
		args = append(args, "--keep-monthly", fmt.Sprint(policy.KeepMonthly))
	}
	if policy.Prune {
		args = append(args, "--prune")
	}
	_, err := r.run(ctx, args...)
	return err
}

func (r *Runner) Check(ctx context.Context) error {
	_, err := r.run(ctx, append([]string{"check"}, lockRetry...)...)
	return err
}
