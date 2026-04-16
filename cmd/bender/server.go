package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/server"
)

const (
	ExitServerAlreadyRunning = 60
	ExitServerNotRunning     = 61
	ExitServerStartFailed    = 62
)

const defaultServerPort = 4317

type serverFlags struct {
	port       int
	projectArg string
	foreground bool
	logPath    string
	pidPath    string
}

func newServerCmd(g *globalFlags) *cobra.Command {
	sf := &serverFlags{}
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run bender-ui, the local web viewer that streams session events",
		Long: `bender server starts the embedded bender-ui HTTP server so you can browse
sessions and watch /ghu --bg runs unfold live at http://localhost:<port>.

By default the server detaches from the terminal and writes its PID + logs
under .bender/ so the shell is not blocked. Use --foreground when you want to
run it under systemd, launchd, or a foreground terminal for debugging.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServerStart(cmd, g, sf)
		},
	}
	cmd.Flags().IntVarP(&sf.port, "port", "p", defaultServerPort, "port to listen on")
	cmd.Flags().StringVar(&sf.projectArg, "project", "", "project root to watch (defaults to current working directory or --project flag from the root command)")
	cmd.Flags().BoolVarP(&sf.foreground, "foreground", "f", false, "run in the foreground instead of detaching")
	cmd.Flags().StringVar(&sf.logPath, "log", "", "log file path (default: <project>/.bender/bender-ui.log)")
	cmd.Flags().StringVar(&sf.pidPath, "pid", "", "pid file path (default: <project>/.bender/bender-ui.pid)")

	cmd.AddCommand(newServerStopCmd(g, sf))
	cmd.AddCommand(newServerStatusCmd(g, sf))
	return cmd
}

func newServerStopCmd(g *globalFlags, sf *serverFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop a detached bender server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveServerProject(g, sf)
			if err != nil {
				return err
			}
			pidFile := resolveServerPidPath(root, sf)
			pid, err := readPidFile(pidFile)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					fmt.Fprintf(cmd.ErrOrStderr(), "server: not running (no pid file at %s)\n", pidFile)
					os.Exit(ExitServerNotRunning)
				}
				return err
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("server: find process %d: %w", pid, err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				if errors.Is(err, os.ErrProcessDone) || strings.Contains(err.Error(), "process already finished") {
					_ = os.Remove(pidFile)
					fmt.Fprintf(cmd.OutOrStdout(), "server: process %d already exited; cleaned up %s\n", pid, pidFile)
					return nil
				}
				return fmt.Errorf("server: SIGTERM %d: %w", pid, err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			_ = os.Remove(pidFile)
			fmt.Fprintf(cmd.OutOrStdout(), "server: stopped (pid=%d)\n", pid)
			return nil
		},
	}
}

func newServerStatusCmd(g *globalFlags, sf *serverFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether a detached bender server is running",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveServerProject(g, sf)
			if err != nil {
				return err
			}
			pidFile := resolveServerPidPath(root, sf)
			pid, err := readPidFile(pidFile)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					fmt.Fprintln(cmd.OutOrStdout(), "server: not running")
					os.Exit(ExitServerNotRunning)
				}
				return err
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				return err
			}
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "server: stale pid file (pid %d no longer alive); remove %s\n", pid, pidFile)
				os.Exit(ExitServerNotRunning)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "server: running (pid=%d, project=%s)\n", pid, root)
			return nil
		},
	}
}

func runServerStart(cmd *cobra.Command, g *globalFlags, sf *serverFlags) error {
	root, err := resolveServerProject(g, sf)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", sf.port)
	pidFile := resolveServerPidPath(root, sf)
	logFile := resolveServerLogPath(root, sf)

	if sf.foreground || os.Getenv("BENDER_UI_DAEMON") == "1" {
		return serveForeground(cmd, root, addr, pidFile)
	}

	// Detach. Refuse to start if a live server is already listening on this port
	// (or the pid file is alive).
	if existing, err := readPidFile(pidFile); err == nil {
		proc, _ := os.FindProcess(existing)
		if proc != nil && proc.Signal(syscall.Signal(0)) == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "server: already running (pid=%d, pid file=%s). Use `bender server stop` first.\n", existing, pidFile)
			os.Exit(ExitServerAlreadyRunning)
		}
		_ = os.Remove(pidFile)
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return fmt.Errorf("server: create log dir: %w", err)
	}

	pid, err := spawnDaemon(root, addr, sf.port, logFile, pidFile)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "server: failed to spawn: %v\n", err)
		os.Exit(ExitServerStartFailed)
	}

	url := fmt.Sprintf("http://localhost:%d/", sf.port)
	fmt.Fprintf(cmd.OutOrStdout(), "bender-ui started\n  pid     %d\n  project %s\n  url     %s\n  log     %s\n  pid     %s\n",
		pid, root, url, logFile, pidFile)
	fmt.Fprintln(cmd.OutOrStdout(), "\nStop with: bender server stop")
	return nil
}

func serveForeground(cmd *cobra.Command, root, addr, pidFile string) error {
	handler, err := server.New(server.Config{ProjectRoot: root, Addr: addr})
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	if err := writePidFile(pidFile, os.Getpid()); err != nil {
		_ = listener.Close()
		return err
	}
	defer os.Remove(pidFile)

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       0, // long-lived SSE
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	fmt.Fprintf(cmd.OutOrStdout(), "bender-ui → http://localhost%s\nproject   → %s\n", addr, root)

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func resolveServerProject(g *globalFlags, sf *serverFlags) (string, error) {
	// Precedence: explicit --project on `server` > root --project > cwd.
	if sf.projectArg != "" {
		return filepath.Abs(sf.projectArg)
	}
	if g.project != "" {
		return resolveProjectRoot(g)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(cwd)
}

func resolveServerPidPath(root string, sf *serverFlags) string {
	if sf.pidPath != "" {
		return sf.pidPath
	}
	return filepath.Join(root, ".bender", "bender-ui.pid")
}

func resolveServerLogPath(root string, sf *serverFlags) string {
	if sf.logPath != "" {
		return sf.logPath
	}
	return filepath.Join(root, ".bender", "bender-ui.log")
}

func writePidFile(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("server: create pid dir: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func readPidFile(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("server: parse pid file %s: %w", path, err)
	}
	return pid, nil
}
