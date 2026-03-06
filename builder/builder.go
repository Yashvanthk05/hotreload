// Package builder runs the user-supplied build command and supports
// cancellation of in-flight builds when a new file change arrives.
package builder

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Builder manages build command execution with cancellation support.
// If a new build is requested while one is running, the running build
// is cancelled first so we always build from the latest state.
type Builder struct {
	cmd    string
	mu     sync.Mutex
	cancel context.CancelFunc
	gen    uint64 // generation counter; incremented on each new build request
}

// New creates a new Builder with the given build command string.
func New(cmd string) *Builder {
	return &Builder{cmd: cmd}
}

// Build executes the build command. If a previous build is still running,
// it is cancelled before starting the new one.
//
// Returns nil on success, or an error if the build failed or context was cancelled.
func (b *Builder) Build(ctx context.Context) error {
	// Cancel any previous in-flight build and capture our generation ID
	b.mu.Lock()
	if b.cancel != nil {
		b.cancel()
	}
	buildCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.gen++
	myGen := b.gen
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		// Only clear the cancel func if it's still ours (same generation)
		if b.gen == myGen {
			b.cancel = nil
		}
		b.mu.Unlock()
		cancel()
	}()

	args := parseCommand(b.cmd)
	if len(args) == 0 {
		return nil
	}

	slog.Info("builder: starting build", "cmd", b.cmd)
	start := time.Now()

	cmd := exec.CommandContext(buildCtx, args[0], args[1:]...)

	// Stream stdout and stderr in real time using slog-wrapped writers
	cmd.Stdout = &slogWriter{level: slog.LevelInfo, prefix: "build"}
	cmd.Stderr = &slogWriter{level: slog.LevelError, prefix: "build"}

	if err := cmd.Run(); err != nil {
		if buildCtx.Err() != nil {
			slog.Info("builder: build cancelled (new change arrived)")
			return buildCtx.Err()
		}
		slog.Error("builder: build failed", "err", err, "duration", time.Since(start))
		return err
	}

	slog.Info("builder: build succeeded", "duration", time.Since(start))
	return nil
}

// parseCommand splits a command string into args, respecting quoted segments.
// This is a simple implementation that handles most common cases.
// For commands with quoted paths containing spaces, the quotes are stripped.
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

// slogWriter is an io.Writer that forwards lines to slog.
type slogWriter struct {
	level  slog.Level
	prefix string
	buf    strings.Builder
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	s := w.buf.String()
	// Flush complete lines
	for {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		line := s[:idx]
		s = s[idx+1:]
		if line != "" {
			slog.Log(context.Background(), w.level, w.prefix+": "+line)
		}
	}
	w.buf.Reset()
	if len(s) > 0 {
		w.buf.WriteString(s)
	}
	return len(p), nil
}
