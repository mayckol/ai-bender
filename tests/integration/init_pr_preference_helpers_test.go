package integration_test

import (
	"os"
	"path/filepath"
)

func mkdirAllForFile(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func readFileBytesHelper(path string) ([]byte, error) {
	return os.ReadFile(path)
}
