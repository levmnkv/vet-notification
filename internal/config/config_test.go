package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success_Defaults(t *testing.T) {
	os.Setenv("BOT_TOKEN", "123:test-token")
	defer os.Unsetenv("BOT_TOKEN")
	// Make sure they are not set to test defaults
	os.Unsetenv("REMINDER_TIME")
	os.Unsetenv("START_DATE")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("TIKV_PD_ADDRS")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "123:test-token", cfg.BotToken)
	assert.Equal(t, 16, cfg.ReminderHour)
	assert.Equal(t, 0, cfg.ReminderMinute)
	assert.Equal(t, "postgres://vet:vet@postgres:5432/vetdb?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, []string{"pd:2379"}, cfg.TiKVPDAddrs)

	expectedDate := time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedDate, cfg.StartDate)
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("BOT_TOKEN", "123:test-token")
	os.Setenv("REMINDER_TIME", "20:30")
	os.Setenv("START_DATE", "2025-05-15")
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	os.Setenv("TIKV_PD_ADDRS", "pd1:2379,pd2:2379")
	defer os.Unsetenv("BOT_TOKEN")
	defer os.Unsetenv("REMINDER_TIME")
	defer os.Unsetenv("START_DATE")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("TIKV_PD_ADDRS")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "123:test-token", cfg.BotToken)
	assert.Equal(t, 20, cfg.ReminderHour)
	assert.Equal(t, 30, cfg.ReminderMinute)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, []string{"pd1:2379", "pd2:2379"}, cfg.TiKVPDAddrs)

	expectedDate := time.Date(2025, 5, 15, 20, 30, 0, 0, time.UTC)
	assert.Equal(t, expectedDate, cfg.StartDate)
}

func TestLoad_NoToken(t *testing.T) {
	os.Unsetenv("BOT_TOKEN")

	cfg, err := Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "BOT_TOKEN")
}
