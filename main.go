// hotreload is a CLI tool that watches a project directory for file changes
// and automatically rebuilds and restarts a server process.
//
// Usage:
//
//	hotreload --root ./myproject --build "go build -o ./bin/server ./cmd/server" --exec "./bin/server"
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hotreload/builder"
	"hotreload/debounce"
	"hotreload/filter"
	"hotreload/runner"
	"hotreload/watcher"

	"github.com/fsnotify/fsnotify"
)

func main() {
	// ── Flag parsing ──────────────────────────────────────────────────────────
	root := flag.String("root", "", "Directory to watch for file changes (required)")
	buildCmd := flag.String("build", "", "Build command (required)")
	execCmd := flag.String("exec", "", "Exec command to run the server (required)")
	flag.Parse()

	if *root == "" || *buildCmd == "" || *execCmd == "" {
		fmt.Fprintf(os.Stderr, "Usage: hotreload --root <dir> --build \"<cmd>\" --exec \"<cmd>\"\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Validate root directory
	if _, err := os.Stat(*root); err != nil {
		slog.Error("hotreload: root directory does not exist", "root", *root, "err", err)
		os.Exit(1)
	}

	// ── Logger setup ─────────────────────────────────────────────────────────
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("hotreload starting",
		"root", *root,
		"build", *buildCmd,
		"exec", *execCmd,
	)

	// ── Signal context ───────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Wire up components ───────────────────────────────────────────────────
	b := builder.New(*buildCmd)
	r := runner.New(*execCmd)
	d := debounce.New(300 * time.Millisecond)

	w, err := watcher.New(*root)
	if err != nil {
		slog.Error("hotreload: failed to create watcher", "err", err)
		os.Exit(1)
	}

	// ── Start watcher goroutine ───────────────────────────────────────────────
	go w.Run(ctx)

	// ── Function to run a full build+restart cycle ────────────────────────────
	rebuild := func() {
		slog.Info("hotreload: triggering rebuild")
		if err := b.Build(ctx); err != nil {
			if ctx.Err() != nil {
				return // We are shutting down
			}
			slog.Error("hotreload: build failed, server not restarted", "err", err)
			return
		}

		// Stop previous server (if any)
		if err := r.Stop(); err != nil {
			slog.Warn("hotreload: error stopping previous server", "err", err)
		}

		// Start new server
		if err := r.Start(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("hotreload: failed to start server", "err", err)
		}
	}

	// ── Trigger initial build immediately (don't wait for a file change) ──────
	go rebuild()

	// ── Main event loop ───────────────────────────────────────────────────────
	slog.Info("hotreload: watching for changes", "root", *root)
	for {
		select {
		case <-ctx.Done():
			slog.Info("hotreload: shutting down")
			d.Stop()
			if err := r.Stop(); err != nil {
				slog.Warn("hotreload: error stopping server during shutdown", "err", err)
			}
			slog.Info("hotreload: goodbye")
			return

		case event, ok := <-w.Events():
			if !ok {
				continue
			}
			if filter.ShouldIgnore(event.Name) {
				continue
			}
			slog.Info("hotreload: file changed",
				"path", event.Name,
				"op", event.Op.String(),
			)
			d.Trigger()

		case _, ok := <-d.C():
			if !ok {
				continue
			}
			// Debounce fired — run rebuild in a goroutine so we don't block
			// the event loop (important: only one rebuild should be in flight
			// because Builder.Build cancels the previous one)
			go rebuild()
		}
	}
}

// Ensure fsnotify is used (it's used indirectly via the watcher package,
// but this import satisfies the go.mod requirement).
var _ = fsnotify.Op(0)
