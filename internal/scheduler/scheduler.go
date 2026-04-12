package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler runs a callback every minute, passing the current UTC hour and minute.
// This allows per-pet reminder times: the callback checks the DB for pets
// whose reminder_time matches the current minute.
type Scheduler struct {
	logger   *slog.Logger
	callback func(ctx context.Context, hour, minute int)
}

// New creates a new Scheduler.
func New(logger *slog.Logger, callback func(ctx context.Context, hour, minute int)) *Scheduler {
	return &Scheduler{
		logger:   logger,
		callback: callback,
	}
}

// Start begins a blocking loop that calls the callback every minute.
// It stops when the context is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("Scheduler started, checking every minute for pending reminders")

	// Align to the next full minute boundary for consistent timing
	now := time.Now().UTC()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	alignDelay := time.Until(nextMinute)

	select {
	case <-ctx.Done():
		s.logger.Info("Scheduler stopped before first tick")
		return
	case <-time.After(alignDelay):
		// Fire immediately on the first aligned minute
		utc := time.Now().UTC()
		s.logger.Info("Scheduler tick", "hour", utc.Hour(), "minute", utc.Minute())
		s.callback(ctx, utc.Hour(), utc.Minute())
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped")
			return
		case t := <-ticker.C:
			utc := t.UTC()
			s.logger.Info("Scheduler tick", "hour", utc.Hour(), "minute", utc.Minute())
			s.callback(ctx, utc.Hour(), utc.Minute())
		}
	}
}
