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
type Builder struct {
	cmd    string
	mu     sync.Mutex
	cancel context.CancelFunc
	gen    uint64 // incremented on each new build request
}

func New(cmd string) *Builder {
	return &Builder{cmd: cmd}
}

func (b *Builder) Build(ctx context.Context) error {
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

type slogWriter struct {
	level  slog.Level
	prefix string
	buf    strings.Builder
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.buf.Write(p)
	s := w.buf.String()
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
