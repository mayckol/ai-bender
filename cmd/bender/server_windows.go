//go:build windows

package main

import (
	"errors"
)

func spawnDaemon(_, _ string, _ int, _, _ string) (int, error) {
	return 0, errors.New("server: detached mode is not supported on Windows yet; run with --foreground and use your service manager (nssm, sc.exe, etc.)")
}
