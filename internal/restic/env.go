package restic

import "fmt"

// RepoConfig is the non-secret part of an R2/S3 restic repository.
type RepoConfig struct {
	Endpoint    string // https://<account>.r2.cloudflarestorage.com
	Bucket      string
	Prefix      string // optional path within the bucket; lets one bucket hold many repos
	AccessKeyID string
}

// BuildEnv returns the environment for a restic invocation.
//
// The S3 secret must be passed at exec time because the S3 backend has no
// command-indirection for it; it is read from the keychain by the caller and
// never written to a file or argv. The repo passphrase, by contrast, is supplied
// via RESTIC_PASSWORD_COMMAND so it never appears in the environment or process
// arguments at all — restic runs the command and reads the passphrase from stdout.
func BuildEnv(base []string, repo RepoConfig, s3Secret, passwordCommand string) []string {
	repoURL := fmt.Sprintf("s3:%s/%s", repo.Endpoint, repo.Bucket)
	if repo.Prefix != "" {
		repoURL += "/" + repo.Prefix
	}
	env := append([]string{}, base...)
	return append(env,
		"RESTIC_REPOSITORY="+repoURL,
		"AWS_ACCESS_KEY_ID="+repo.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+s3Secret,
		"AWS_DEFAULT_REGION=auto",
		"RESTIC_PASSWORD_COMMAND="+passwordCommand,
	)
}
