// Package watcher provides recursive directory watching using fsnotify.
// It handles dynamic addition of newly created directories and gracefully
// ignores removed directories.
package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"hotreload/filter"

	"github.com/fsnotify/fsnotify"
)

const maxWatchDirs = 500

// Watcher recursively watches a root directory and all subdirectories.
// It dynamically adds new directories as they are created, and logs
// when directories are removed.
type Watcher struct {
	root    string
	fsw     *fsnotify.Watcher
	eventCh chan fsnotify.Event
}

// New creates a new Watcher for the given root directory.
// It walks all subdirectories (excluding ignored paths) and begins watching them.
func New(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		root:    root,
		fsw:     fsw,
		eventCh: make(chan fsnotify.Event, 100),
	}

	if err := w.addDirRecursive(root); err != nil {
		fsw.Close()
		return nil, err
	}

	return w, nil
}

// addDirRecursive walks the given directory tree and adds all non-ignored
// directories to the fsnotify watcher.
func (w *Watcher) addDirRecursive(root string) error {
	dirCount := 0
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("watcher: walk error", "path", path, "err", err)
			return nil // Continue walking
		}

		if !d.IsDir() {
			return nil
		}

		// Skip ignored directories
		if filter.IsIgnoredDir(path) {
			slog.Debug("watcher: skipping ignored dir", "path", path)
			return filepath.SkipDir
		}

		dirCount++
		if dirCount > maxWatchDirs {
			slog.Warn("watcher: directory count exceeds limit — consider raising ulimit -n",
				"limit", maxWatchDirs,
				"hint", "run: ulimit -n 65536")
		}

		if err := w.fsw.Add(path); err != nil {
			slog.Warn("watcher: failed to watch dir", "path", path, "err", err)
		} else {
			slog.Debug("watcher: watching dir", "path", path)
		}
		return nil
	})
}

// Run starts the watcher event loop, forwarding relevant file events to
// the channel returned by Events(). It blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	defer w.fsw.Close()
	defer close(w.eventCh)

	for {
		select {
		case <-ctx.Done():
			slog.Info("watcher: shutting down")
			return

		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Error("watcher: fsnotify error", "err", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// If a new directory was created, start watching it
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if !filter.IsIgnoredDir(path) {
				slog.Info("watcher: new directory detected, watching", "path", path)
				if err := w.addDirRecursive(path); err != nil {
					slog.Warn("watcher: failed to add new dir", "path", path, "err", err)
				}
			}
			// Don't forward directory creation events as file events
			return
		}
	}

	// If a directory was removed, log it (fsnotify auto-removes it)
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		info, err := os.Stat(path)
		if err != nil || (err == nil && info.IsDir()) {
			slog.Debug("watcher: directory removed or renamed", "path", path)
		}
	}

	// Skip files that should be ignored
	if filter.ShouldIgnore(path) {
		slog.Debug("watcher: ignoring file event", "path", path, "op", event.Op)
		return
	}

	slog.Debug("watcher: file event", "path", path, "op", event.Op)

	// Forward to event channel (non-blocking: drop if consumer is slow)
	select {
	case w.eventCh <- event:
	default:
		slog.Debug("watcher: event channel full, dropping event", "path", path)
	}
}

// Events returns the channel of file events that passed filtering.
func (w *Watcher) Events() <-chan fsnotify.Event {
	return w.eventCh
}
