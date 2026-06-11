// Package secret stores the two dotbackup secrets — the restic repo passphrase
// and the S3 secret access key — in the OS keychain. They never touch config.json,
// the launchd plist, or argv.
package secret

import "github.com/zalando/go-keyring"

const (
	servicePassphrase = "dotbackup-passphrase"
	serviceS3Secret   = "dotbackup-s3-secret"
)

func StorePassphrase(user, passphrase string) error {
	return keyring.Set(servicePassphrase, user, passphrase)
}

func ReadPassphrase(user string) (string, error) {
	return keyring.Get(servicePassphrase, user)
}

func DeletePassphrase(user string) error {
	return keyring.Delete(servicePassphrase, user)
}

func StoreS3Secret(user, secret string) error {
	return keyring.Set(serviceS3Secret, user, secret)
}

func ReadS3Secret(user string) (string, error) {
	return keyring.Get(serviceS3Secret, user)
}

func DeleteS3Secret(user string) error {
	return keyring.Delete(serviceS3Secret, user)
}
