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

type Watcher struct {
	root    string
	fsw     *fsnotify.Watcher
	eventCh chan fsnotify.Event
}

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

func (w *Watcher) addDirRecursive(root string) error {
	dirCount := 0
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("watcher: walk error", "path", path, "err", err)
			return nil
		}

		if !d.IsDir() {
			return nil
		}

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

func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if !filter.IsIgnoredDir(path) {
				slog.Info("watcher: new directory detected, watching", "path", path)
				if err := w.addDirRecursive(path); err != nil {
					slog.Warn("watcher: failed to add new dir", "path", path, "err", err)
				}
			}
			return
		}
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		info, err := os.Stat(path)
		if err != nil || (err == nil && info.IsDir()) {
			slog.Debug("watcher: directory removed or renamed", "path", path)
		}
	}

	if filter.ShouldIgnore(path) {
		slog.Debug("watcher: ignoring file event", "path", path, "op", event.Op)
		return
	}

	slog.Debug("watcher: file event", "path", path, "op", event.Op)

	select {
	case w.eventCh <- event:
	default:
		slog.Debug("watcher: event channel full, dropping event", "path", path)
	}
}

func (w *Watcher) Events() <-chan fsnotify.Event {
	return w.eventCh
}
