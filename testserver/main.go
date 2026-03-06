// Simple HTTP test server for demonstrating hotreload.
// Responds to GET / with a message including the current PID.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	pid := os.Getpid()

	// Version tag — edit this to see the reload in action!
	version := "v1.0.0"

	slog.Info("testserver starting",
		"pid", pid,
		"addr", addr,
		"version", version,
		"started_at", time.Now().Format(time.RFC3339),
	)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("testserver: request received",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		fmt.Fprintf(w, "Hello from testserver! PID: %d | Version: %s | Time: %s\n",
			pid, version, time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","pid":%d}`, pid)
	})

	slog.Info("testserver: listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("testserver: fatal error", "err", err)
		os.Exit(1)
	}
}
