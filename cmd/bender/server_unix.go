//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// spawnDaemon starts a new bender process in detached mode and returns its pid.
// The child runs `bender server --foreground --port <N> --project <root> --pid <path> --log <path>`
// with BENDER_UI_DAEMON=1 in its environment. Setsid detaches it from the
// controlling terminal so the user's shell returns immediately.
func spawnDaemon(root, _ string, port int, logFile, pidFile string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("server: resolve executable: %w", err)
	}

	args := []string{
		"server",
		"--foreground",
		"--port", strconv.Itoa(port),
		"--project", root,
		"--pid", pidFile,
		"--log", logFile,
	}

	out, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("server: open log file %s: %w", logFile, err)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "BENDER_UI_DAEMON=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		_ = out.Close()
		return 0, fmt.Errorf("server: start daemon: %w", err)
	}
	// cmd.Start has duped the fd; close our copy so the daemon owns the log.
	_ = out.Close()

	pid := cmd.Process.Pid
	// Release so the child isn't a zombie when we exit immediately.
	if err := cmd.Process.Release(); err != nil {
		return pid, fmt.Errorf("server: release child: %w", err)
	}

	// Give the child a moment to write its pid file. If it crashed early,
	// the pid file will be missing and the caller can surface it.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(pidFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return pid, nil
}
