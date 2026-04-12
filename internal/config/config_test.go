package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success_Defaults(t *testing.T) {
	os.Setenv("BOT_TOKEN", "123:test-token")
	defer os.Unsetenv("BOT_TOKEN")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("TIKV_PD_ADDRS")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "123:test-token", cfg.BotToken)
	assert.Equal(t, "postgres://vet:vet@postgres:5432/vetdb?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, []string{"pd:2379"}, cfg.TiKVPDAddrs)
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("BOT_TOKEN", "123:test-token")
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	os.Setenv("TIKV_PD_ADDRS", "pd1:2379,pd2:2379")
	defer os.Unsetenv("BOT_TOKEN")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("TIKV_PD_ADDRS")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "123:test-token", cfg.BotToken)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, []string{"pd1:2379", "pd2:2379"}, cfg.TiKVPDAddrs)
}

func TestLoad_NoToken(t *testing.T) {
	os.Unsetenv("BOT_TOKEN")

	cfg, err := Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "BOT_TOKEN")
}
