package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BotToken       string
	ReminderHour   int
	ReminderMinute int
	StartDate      time.Time
	DatabaseURL    string
	TiKVPDAddrs    []string
}

func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}

	reminderTimeStr := os.Getenv("REMINDER_TIME")
	reminderHour := 16
	reminderMinute := 0
	if reminderTimeStr != "" {
		parts := strings.Split(reminderTimeStr, ":")
		if len(parts) == 2 {
			if h, err := strconv.Atoi(parts[0]); err == nil && h >= 0 && h <= 23 {
				reminderHour = h
			}
			if m, err := strconv.Atoi(parts[1]); err == nil && m >= 0 && m <= 59 {
				reminderMinute = m
			}
		} else {
			return nil, fmt.Errorf("invalid REMINDER_TIME format, expected HH:MM")
		}
	}

	startDateStr := os.Getenv("START_DATE")
	startDate := time.Date(2026, 3, 8, reminderHour, reminderMinute, 0, 0, time.UTC)
	if startDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid START_DATE format, expected YYYY-MM-DD: %w", err)
		}
		// Combine date with time
		startDate = time.Date(
			parsedDate.Year(), parsedDate.Month(), parsedDate.Day(),
			reminderHour, reminderMinute, 0, 0, time.UTC,
		)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://vet:vet@postgres:5432/vetdb?sslmode=disable"
	}

	tikvAddrsStr := os.Getenv("TIKV_PD_ADDRS")
	var tikvPDAddrs []string
	if tikvAddrsStr == "" {
		tikvPDAddrs = []string{"pd:2379"}
	} else {
		for _, addr := range strings.Split(tikvAddrsStr, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				tikvPDAddrs = append(tikvPDAddrs, addr)
			}
		}
	}

	return &Config{
		BotToken:       token,
		ReminderHour:   reminderHour,
		ReminderMinute: reminderMinute,
		StartDate:      startDate,
		DatabaseURL:    databaseURL,
		TiKVPDAddrs:    tikvPDAddrs,
	}, nil
}
