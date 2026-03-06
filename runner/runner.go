// Package runner starts and stops the server process, killing the entire
// process group to ensure no orphan children remain. It also implements
// crash loop protection with exponential back-off.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// crashThreshold is how long a process must run before it's considered stable.
	crashThreshold = 1 * time.Second

	// stableThreshold is how long a process must run to reset the back-off counter.
	stableThreshold = 5 * time.Second

	// maxBackoff is the maximum wait time before restarting after a crash.
	maxBackoff = 30 * time.Second

	// initialBackoff is the base back-off duration.
	initialBackoff = 500 * time.Millisecond
)

// Runner manages the lifecycle of the server process.
type Runner struct {
	cmdStr     string
	mu         sync.Mutex
	proc       *os.Process
	pgid       int
	crashCount int
	lastStart  time.Time
}

// New creates a new Runner with the given exec command string.
func New(cmdStr string) *Runner {
	return &Runner{cmdStr: cmdStr}
}

// Stop terminates the current server process and its entire process group.
// It is safe to call Stop even if the server is not running.
func (r *Runner) Stop() error {
	r.mu.Lock()
	proc := r.proc
	pgid := r.pgid
	r.proc = nil
	r.pgid = 0
	r.mu.Unlock()

	if proc == nil {
		return nil
	}

	slog.Info("runner: stopping server", "pid", proc.Pid)

	// First try graceful shutdown with SIGTERM to the process group
	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = proc.Signal(syscall.SIGTERM)
	}

	// Give it a short window to exit gracefully
	done := make(chan struct{})
	go func() {
		proc.Wait() //nolint:errcheck
		close(done)
	}()

	select {
	case <-done:
		slog.Info("runner: server stopped gracefully", "pid", proc.Pid)
	case <-time.After(2 * time.Second):
		// Force kill the entire process group
		slog.Warn("runner: server did not stop gracefully, force killing process group", "pgid", pgid)
		if pgid > 0 {
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
				slog.Error("runner: failed to SIGKILL process group", "pgid", pgid, "err", err)
			}
		} else {
			proc.Kill() //nolint:errcheck
		}
		<-done
		slog.Info("runner: server force-killed", "pid", proc.Pid)
	}

	return nil
}

// Start launches the server process. If a crash loop is detected (process
// exits within crashThreshold), it applies exponential back-off before
// returning an error. The caller should retry after a delay.
//
// The server's stdout and stderr are piped directly to os.Stdout/os.Stderr
// for real-time log streaming.
func (r *Runner) Start(ctx context.Context) error {
	args := parseCommand(r.cmdStr)
	if len(args) == 0 {
		return fmt.Errorf("runner: empty exec command")
	}

	// Apply crash loop back-off if needed
	r.mu.Lock()
	backoffDelay := r.backoffDelay()
	r.mu.Unlock()

	if backoffDelay > 0 {
		slog.Warn("runner: server crashed immediately — backing off before retry",
			"backoff", backoffDelay.Round(time.Millisecond))
		select {
		case <-time.After(backoffDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Put the process in its own process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Stream logs in real time — no buffering
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("runner: failed to start server: %w", err)
	}

	pid := cmd.Process.Pid
	// Get the process group ID (on Linux with Setpgid=true, pgid == pid)
	pgid := pid

	r.mu.Lock()
	r.proc = cmd.Process
	r.pgid = pgid
	r.lastStart = time.Now()
	r.mu.Unlock()

	slog.Info("runner: server started", "pid", pid, "cmd", r.cmdStr)

	// Monitor process exit in background — for crash detection
	go func() {
		err := cmd.Wait()
		uptime := time.Since(r.lastStart)

		r.mu.Lock()
		// Check if this is still our process (not already replaced)
		isCurrent := r.proc == cmd.Process
		if isCurrent {
			r.proc = nil
			r.pgid = 0

			if uptime < crashThreshold {
				r.crashCount++
				slog.Warn("runner: server crashed immediately",
					"pid", pid, "uptime", uptime.Round(time.Millisecond),
					"crash_count", r.crashCount)
			} else if uptime >= stableThreshold {
				if r.crashCount > 0 {
					slog.Info("runner: server ran stably, resetting crash counter",
						"pid", pid, "uptime", uptime.Round(time.Millisecond))
				}
				r.crashCount = 0
			}
		}
		r.mu.Unlock()

		if err != nil && ctx.Err() == nil {
			slog.Error("runner: server exited with error", "pid", pid, "err", err, "uptime", uptime.Round(time.Millisecond))
		} else {
			slog.Info("runner: server exited", "pid", pid, "uptime", uptime.Round(time.Millisecond))
		}
	}()

	return nil
}

// backoffDelay returns how long to wait before starting after a crash.
// Must be called with r.mu held.
func (r *Runner) backoffDelay() time.Duration {
	if r.crashCount == 0 {
		return 0
	}
	// Exponential back-off: 500ms, 1s, 2s, 4s, 8s, ... capped at 30s
	delay := initialBackoff
	for i := 1; i < r.crashCount; i++ {
		delay *= 2
		if delay > maxBackoff {
			delay = maxBackoff
			break
		}
	}
	return delay
}

// parseCommand splits a command string into args using simple space-splitting,
// respecting single and double quotes.
func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
		case !inQuote && r == ' ':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
