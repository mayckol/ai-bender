package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func makeDir(t *testing.T, path string) error {
	t.Helper()
	return os.MkdirAll(path, 0o755)
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func splitLines(data []byte) [][]byte {
	data = bytes.TrimRight(data, "\n")
	if len(data) == 0 {
		return nil
	}
	return bytes.Split(data, []byte("\n"))
}

func itoa(i int) string { return strconv.Itoa(i) }

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		i, err := x.Int64()
		if err == nil {
			return int(i), true
		}
		f, err := x.Float64()
		if err == nil {
			return int(f), true
		}
	}
	return 0, false
}
