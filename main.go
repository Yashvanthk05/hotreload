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
	root := flag.String("root", "", "Directory to watch for file changes (required)")
	buildCmd := flag.String("build", "", "Build command (required)")
	execCmd := flag.String("exec", "", "Exec command to run the server (required)")
	flag.Parse()

	if *root == "" || *buildCmd == "" || *execCmd == "" {
		fmt.Fprintf(os.Stderr, "Usage: hotreload --root <dir> --build \"<cmd>\" --exec \"<cmd>\"\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if _, err := os.Stat(*root); err != nil {
		slog.Error("hotreload: root directory does not exist", "root", *root, "err", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("hotreload starting",
		"root", *root,
		"build", *buildCmd,
		"exec", *execCmd,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	b := builder.New(*buildCmd)
	r := runner.New(*execCmd)
	d := debounce.New(300 * time.Millisecond)

	w, err := watcher.New(*root)
	if err != nil {
		slog.Error("hotreload: failed to create watcher", "err", err)
		os.Exit(1)
	}

	go w.Run(ctx)

	rebuild := func() {
		slog.Info("hotreload: triggering rebuild")
		if err := b.Build(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("hotreload: build failed, server not restarted", "err", err)
			return
		}

		if err := r.Stop(); err != nil {
			slog.Warn("hotreload: error stopping previous server", "err", err)
		}

		if err := r.Start(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("hotreload: failed to start server", "err", err)
		}
	}

	go rebuild()

	slog.Info("hotreload: watching for changes", "root", *root)
	for {
		select {
		case <-ctx.Done():
			slog.Info("hotreload: shutting down")
			d.Stop()
			if err := r.Stop(); err != nil {
				slog.Warn("hotreload: error stopping server during shutdown", "err", err)
			}
			slog.Info("hotreload: stopped")
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
			go rebuild()
		}
	}
}

var _ = fsnotify.Op(0)
