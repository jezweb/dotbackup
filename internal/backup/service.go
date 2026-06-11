// Package backup orchestrates the engine from saved config + keychain: building a
// Runner, deciding which folders are due, and running backups. Phase 1 covers the
// Runner construction the UI and the scheduled runner both depend on.
package backup

import (
	"fmt"
	"os"

	"github.com/jezweb/dotbackup/internal/config"
	"github.com/jezweb/dotbackup/internal/restic"
	"github.com/jezweb/dotbackup/internal/secret"
)

// ResticBin resolves the restic binary. Dev uses an override or the homebrew path;
// the shipped app points this at the vendored binary inside the app bundle.
func ResticBin() string {
	if v := os.Getenv("DOTBACKUP_RESTIC"); v != "" {
		return v
	}
	return "/opt/homebrew/bin/restic"
}

// NewRunner builds an engine Runner from saved config plus the keychain. The
// passwordCmd is what restic invokes to read the passphrase (the app binary's
// --print-passphrase mode); it inherits DOTBACKUP_USER so it can find the entry.
func NewRunner(cfg *config.Config, passwordCmd string) (*restic.Runner, error) {
	s3secret, err := secret.ReadS3Secret(cfg.User)
	if err != nil {
		return nil, fmt.Errorf("read S3 secret from keychain: %w", err)
	}
	repo := restic.RepoConfig{
		Endpoint:    cfg.Repo.Endpoint,
		Bucket:      cfg.Repo.Bucket,
		Prefix:      cfg.Repo.Prefix,
		AccessKeyID: cfg.Repo.AccessKeyID,
	}
	base := []string{"DOTBACKUP_USER=" + cfg.User, "HOME=" + os.Getenv("HOME")}
	return &restic.Runner{
		Bin: ResticBin(),
		Env: restic.BuildEnv(base, repo, s3secret, passwordCmd),
	}, nil
}
