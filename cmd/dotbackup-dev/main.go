// Command dotbackup-dev exercises the engine wrapper against a real R2 repo.
// It is a development harness for the Slice 1 verification gate, not the shipped
// app. Two subcommands:
//
//	print-passphrase   reads the keychain and writes the repo passphrase to stdout
//	                   (this is what restic's RESTIC_PASSWORD_COMMAND invokes)
//	gate               init → backup → snapshots → ls → restore → byte-compare
//
// Repo coordinates come from env; the S3 secret and passphrase come from the
// keychain, exactly as the shipped app will read them.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jezweb/dotbackup/internal/backup"
	"github.com/jezweb/dotbackup/internal/config"
	"github.com/jezweb/dotbackup/internal/restic"
	"github.com/jezweb/dotbackup/internal/secret"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dotbackup-dev <print-passphrase|gate>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "print-passphrase":
		pass, err := secret.ReadPassphrase(mustEnv("DOTBACKUP_USER"))
		check(err)
		fmt.Print(pass)
	case "gate":
		gate()
	case "init":
		initRepo()
	case "validate":
		validate()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func gate() {
	ctx := context.Background()
	self, err := os.Executable()
	check(err)
	user := mustEnv("DOTBACKUP_USER")

	s3secret, err := secret.ReadS3Secret(user)
	check(err)
	repo := restic.RepoConfig{
		Endpoint:    mustEnv("DOTBACKUP_ENDPOINT"),
		Bucket:      mustEnv("DOTBACKUP_BUCKET"),
		Prefix:      os.Getenv("DOTBACKUP_PREFIX"),
		AccessKeyID: mustEnv("DOTBACKUP_ACCESS_KEY_ID"),
	}
	base := []string{"DOTBACKUP_USER=" + user, "HOME=" + os.Getenv("HOME")}
	pwCmd := self + " print-passphrase"
	runner := &restic.Runner{
		Bin: envOr("DOTBACKUP_RESTIC", "/opt/homebrew/bin/restic"),
		Env: restic.BuildEnv(base, repo, s3secret, pwCmd),
	}

	fmt.Println("→ init")
	check(runner.Init(ctx))

	// Build a source tree with known content; resolve symlinks so the path restic
	// stores matches what we pass to --include on restore (macOS /var → /private/var).
	tmp, err := os.MkdirTemp("", "dotbackup-src-")
	check(err)
	src, err := filepath.EvalSymlinks(tmp)
	check(err)
	defer os.RemoveAll(src)

	want := append([]byte("hello dotbackup gate "+time.Now().Format(time.RFC3339Nano)+"\n"), randomBytes(4096)...)
	alpha := filepath.Join(src, "alpha.txt")
	check(os.WriteFile(alpha, want, 0o644))
	check(os.WriteFile(filepath.Join(src, "beta.txt"), []byte("second file"), 0o644))
	check(os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	check(os.WriteFile(filepath.Join(src, "nested", "gamma.txt"), []byte("nested file"), 0o644))

	fmt.Println("→ backup")
	var maxPct float64
	var progressEvents int
	summary, err := runner.Backup(ctx, []string{src}, "", func(s restic.BackupStatus) {
		progressEvents++
		if s.PercentDone > maxPct {
			maxPct = s.PercentDone
		}
	})
	check(err)
	fmt.Printf("  snapshot=%s files_new=%d data_added=%dB progress_events=%d max_pct=%.2f\n",
		short(summary.SnapshotID), summary.FilesNew, summary.DataAdded, progressEvents, maxPct)
	if summary.SnapshotID == "" {
		fail("summary carried no snapshot_id")
	}

	snaps, err := runner.Snapshots(ctx)
	check(err)
	fmt.Printf("→ snapshots: %d\n", len(snaps))
	if len(snaps) == 0 {
		fail("no snapshots listed after backup")
	}
	snapID := snaps[len(snaps)-1].ID

	nodes, err := runner.Ls(ctx, snapID)
	check(err)
	var files int
	for _, n := range nodes {
		if n.Type == "file" {
			files++
		}
	}
	fmt.Printf("→ ls: %d nodes, %d files\n", len(nodes), files)
	if files < 3 {
		fail(fmt.Sprintf("expected >=3 files in snapshot, ls saw %d", files))
	}

	dst, err := os.MkdirTemp("", "dotbackup-restore-")
	check(err)
	defer os.RemoveAll(dst)
	fmt.Println("→ restore alpha.txt only")
	check(runner.Restore(ctx, snapID, dst, []string{alpha}, nil))

	restored := filepath.Join(dst, alpha) // restic recreates the absolute path under target
	got, err := os.ReadFile(restored)
	check(err)
	if !bytes.Equal(got, want) {
		fail(fmt.Sprintf("restored bytes differ: got %d, want %d", len(got), len(want)))
	}
	h1, h2 := sha256.Sum256(want), sha256.Sum256(got)
	if h1 != h2 {
		fail("sha256 mismatch")
	}
	fmt.Printf("✓ restored byte-identical (%d bytes, sha256 %s…)\n", len(got), hex.EncodeToString(h1[:6]))

	// Selective restore must NOT bring beta.txt.
	if _, err := os.Stat(filepath.Join(dst, filepath.Join(src, "beta.txt"))); err == nil {
		fail("selective restore leaked beta.txt")
	}
	fmt.Println("✓ selective restore excluded beta.txt")

	fmt.Println("\nGATE 1 PASS ✓  init · backup(stream) · snapshots · ls · selective restore · byte-identical")
}

// initRepo mirrors the setup skill's `restic init` step: load the config the
// skill wrote, build the runner from it + the keychain, initialise the repo.
func initRepo() {
	self, err := os.Executable()
	check(err)
	cfg, err := config.Load()
	check(err)
	runner, err := backup.NewRunner(cfg, self+" print-passphrase")
	check(err)
	check(runner.Init(context.Background()))
	fmt.Printf("✓ repo initialised: s3:%s/%s/%s\n", cfg.Repo.Endpoint, cfg.Repo.Bucket, cfg.Repo.Prefix)
}

// validate mirrors the app launch path: read the saved config, reach the engine,
// list snapshots. Proves the config the skill writes drives restic.
func validate() {
	self, err := os.Executable()
	check(err)
	cfg, err := config.Load()
	check(err)
	fmt.Printf("→ config: user=%s bucket=%s prefix=%s folders=%d\n",
		cfg.User, cfg.Repo.Bucket, cfg.Repo.Prefix, len(cfg.Folders))
	runner, err := backup.NewRunner(cfg, self+" print-passphrase")
	check(err)
	snaps, err := runner.Snapshots(context.Background())
	check(err)
	fmt.Printf("✓ engine reachable from saved config — %d snapshot(s)\n", len(snaps))
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	// keep it printable-ish so a human eyeballing the object sees noise, not control chars
	for i := range b {
		b[i] = 'A' + (b[i] % 26)
	}
	return b
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fail("missing required env " + k)
	}
	return v
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "GATE FAIL ✗ "+msg)
	os.Exit(1)
}
