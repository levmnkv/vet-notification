package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler handles executing a callback at a specific time every day.
type Scheduler struct {
	logger   *slog.Logger
	callback func(ctx context.Context)
	hour     int
	minute   int
}

// New creates a new Scheduler that runs every day at the given hour and minute UTC.
func New(hour, minute int, logger *slog.Logger, callback func(ctx context.Context)) *Scheduler {
	return &Scheduler{
		logger:   logger,
		callback: callback,
		hour:     hour,
		minute:   minute,
	}
}

// GetNextTargetTime calculates the next target time.
func GetNextTargetTime(now time.Time, targetHour, targetMinute int) time.Time {
	nowUTC := now.UTC()
	targetTime := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), targetHour, targetMinute, 0, 0, time.UTC)

	if targetTime.Before(nowUTC) || targetTime.Equal(nowUTC) {
		targetTime = targetTime.AddDate(0, 0, 1) // Next day
	}

	return targetTime
}

// Start begins a blocking loop that calls the callback at the scheduled time.
// It stops when the context is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	for {
		now := time.Now()
		targetTime := GetNextTargetTime(now, s.hour, s.minute)

		s.logger.Info("Scheduler waiting for next execution", "next_run", targetTime.Format(time.RFC3339))

		// Check the clock every 30 seconds to be resilient against OS hibernation/sleep
		ticker := time.NewTicker(30 * time.Second)

	waiting:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				s.logger.Info("Scheduler stopped")
				return
			case t := <-ticker.C:
				if !t.UTC().Before(targetTime) {
					ticker.Stop()
					break waiting
				}
			}
		}

		s.logger.Info("Executing scheduled task")
		s.callback(ctx)

		// To prevent double execution if the callback is very fast, sleep past the target time second
		time.Sleep(1 * time.Second)
	}
}
