package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BotToken    string
	DatabaseURL string
	TiKVPDAddrs []string
}

func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
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
		BotToken:    token,
		DatabaseURL: databaseURL,
		TiKVPDAddrs: tikvPDAddrs,
	}, nil
}
