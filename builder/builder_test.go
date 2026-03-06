package builder_test

import (
	"context"
	"testing"
	"time"

	"hotreload/builder"
)

func TestBuilder_SuccessfulBuild(t *testing.T) {
	b := builder.New("echo hello")
	ctx := context.Background()

	if err := b.Build(ctx); err != nil {
		t.Fatalf("expected successful build, got: %v", err)
	}
}

func TestBuilder_FailedBuild(t *testing.T) {
	b := builder.New("false") // 'false' always exits with code 1
	ctx := context.Background()

	if err := b.Build(ctx); err == nil {
		t.Fatal("expected build to fail, but it succeeded")
	}
}

func TestBuilder_CancelledContext_AbortsRun(t *testing.T) {
	// Use 'sleep 10' as a long-running build command
	b := builder.New("sleep 10")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- b.Build(ctx)
	}()

	// Cancel the context after a short wait
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after cancellation, got nil")
		}
		// The error should be context.Canceled or a signal error
	case <-time.After(3 * time.Second):
		t.Fatal("Build did not abort within expected time after context cancellation")
	}
}

func TestBuilder_EmptyCommand(t *testing.T) {
	b := builder.New("")
	ctx := context.Background()

	// Empty command should return nil (no-op)
	if err := b.Build(ctx); err != nil {
		t.Fatalf("empty build command should be a no-op, got: %v", err)
	}
}

func TestBuilder_QuotedArgs(t *testing.T) {
	// Test that quoted arguments are handled correctly
	// 'echo "hello world"' should produce one arg "hello world"
	b := builder.New(`echo "hello world"`)
	ctx := context.Background()

	if err := b.Build(ctx); err != nil {
		t.Fatalf("quoted build command failed: %v", err)
	}
}
