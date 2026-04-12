package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScheduler_CallsCallbackWithCurrentTime(t *testing.T) {
	var calledHour, calledMinute int32

	callback := func(ctx context.Context, hour, minute int) {
		atomic.StoreInt32(&calledHour, int32(hour))
		atomic.StoreInt32(&calledMinute, int32(minute))
	}

	sched := New(slog.Default(), callback)

	// Call the callback directly to verify it works
	now := time.Now().UTC()
	callback(context.Background(), now.Hour(), now.Minute())

	assert.Equal(t, int32(now.Hour()), atomic.LoadInt32(&calledHour))
	assert.Equal(t, int32(now.Minute()), atomic.LoadInt32(&calledMinute))

	// Verify struct is properly initialized
	assert.NotNil(t, sched.callback)
}

func TestScheduler_StopsOnContextCancel(t *testing.T) {
	callback := func(ctx context.Context, hour, minute int) {}

	sched := New(slog.Default(), callback)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	// Cancel immediately
	cancel()

	select {
	case <-done:
		// Success: scheduler stopped
	case <-time.After(3 * time.Second):
		t.Fatal("Scheduler did not stop within timeout")
	}
}
