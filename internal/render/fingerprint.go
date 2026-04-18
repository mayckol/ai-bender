package render

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// FingerprintFileName is the hidden sidecar where init records the SHA-256
// of the last-generated `.bender/pipeline.yaml`, relative to workspace root.
const FingerprintFileName = ".bender/.pipeline.fingerprint"

// Fingerprint returns the lowercase-hex SHA-256 of the given bytes. Used to
// compare "what init would write now" against "what's on disk" to decide
// whether the user has edited the file.
func Fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteFingerprint writes the sidecar to workspaceRoot, creating the
// `.bender/` dir if needed.
func WriteFingerprint(workspaceRoot, hex string) error {
	path := filepath.Join(workspaceRoot, FingerprintFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(hex), 0o644)
}

// ReadFingerprint returns the sidecar's recorded hex (empty string if
// absent — callers treat "no sidecar" as "user-edited" by default).
func ReadFingerprint(workspaceRoot string) (string, error) {
	path := filepath.Join(workspaceRoot, FingerprintFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// IsDrifted returns true when the pipeline.yaml at pipelinePath differs
// from the bytes whose fingerprint was recorded in the sidecar. A missing
// sidecar is treated as "user-edited" (safe default).
func IsDrifted(workspaceRoot, pipelinePath string) (bool, error) {
	recorded, err := ReadFingerprint(workspaceRoot)
	if err != nil {
		return true, err
	}
	data, err := os.ReadFile(pipelinePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil // no file, no drift
		}
		return true, err
	}
	if recorded == "" {
		return true, nil
	}
	return Fingerprint(data) != recorded, nil
}
