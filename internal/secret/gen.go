package secret

import (
	"crypto/rand"
	"encoding/base64"
)

// GeneratePassphrase returns a strong random repo passphrase. This is the only
// key to the encrypted backup — it is shown to the user once at setup and stored
// in the keychain; it is never recoverable if lost.
func GeneratePassphrase() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
