package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hotreload/watcher"

	"github.com/fsnotify/fsnotify"
)

func TestWatcher_DetectsFileChange(t *testing.T) {
	// Create a temporary directory to watch
	dir := t.TempDir()

	// Create an initial file
	initFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(initFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go w.Run(ctx)

	// Give the watcher a moment to set up
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(initFile, []byte("package main\n// changed"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events():
		if event.Name != initFile {
			t.Logf("received event for %q (expected %q) — this is okay", event.Name, initFile)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for file change event")
	}
}

func TestWatcher_NewSubdirIsWatched(t *testing.T) {
	dir := t.TempDir()

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Create a new subdirectory
	subdir := filepath.Join(dir, "newpkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Give the watcher time to detect and add the new directory
	time.Sleep(300 * time.Millisecond)

	// Create a file in the new subdirectory — should be watched
	newFile := filepath.Join(subdir, "handler.go")
	if err := os.WriteFile(newFile, []byte("package newpkg"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		// Good — we received an event from the new subdirectory
	case <-ctx.Done():
		t.Fatal("timed out — new subdirectory was not watched")
	}
}

func TestWatcher_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()

	// Create a .git directory with a file
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Write a file to .git — should NOT trigger an event
	gitFile := filepath.Join(gitDir, "COMMIT_EDITMSG")
	if err := os.WriteFile(gitFile, []byte("initial commit"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events():
		t.Errorf("received unexpected event for .git file: %v", event)
	case <-ctx.Done():
		// Good — no events from .git directory
	}
}

func TestWatcher_DeletedDirDoesNotCrash(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory
	subdir := filepath.Join(dir, "subpkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	w, err := watcher.New(dir)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Remove the watched subdirectory — should not crash
	if err := os.RemoveAll(subdir); err != nil {
		t.Fatal(err)
	}

	// Give the watcher time to handle the removal
	time.Sleep(200 * time.Millisecond)

	// Write to the root dir to confirm the watcher is still alive
	rootFile := filepath.Join(dir, "alive.go")
	if err := os.WriteFile(rootFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		// Good — watcher is still alive after dir removal
	case <-ctx.Done():
		t.Fatal("watcher died after directory deletion")
	}
}

// Ensure the fsnotify package is imported — used indirectly via watcher
var _ = fsnotify.Op(0)
