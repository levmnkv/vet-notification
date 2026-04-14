package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/bot"
	"github.com/levmi/vet-notifications-go/internal/config"
	"github.com/levmi/vet-notifications-go/internal/kv"
	"github.com/levmi/vet-notifications-go/internal/logger"
	"github.com/levmi/vet-notifications-go/internal/scheduler"
	"github.com/levmi/vet-notifications-go/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("Failed to load configuration: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.Setup(cfg.BotToken)
	log.Info("Starting Vet Notifications Bot...")

	st, err := storage.NewPostgresStorage(cfg.DatabaseURL)
	if err != nil {
		log.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	kvStore, err := kv.NewTiKVStore(cfg.TiKVPDAddrs)
	if err != nil {
		log.Error("Failed to initialize TiKV store", "error", err)
		os.Exit(1)
	}
	defer kvStore.Close()

	// Redirect tgbotapi logs through our token mask writer
	_ = tgbotapi.SetLogger(stdlog.New(logger.NewTokenMaskWriter(log, cfg.BotToken), "", 0))

	b, err := bot.New(cfg.BotToken, log, st, kvStore)
	if err != nil {
		log.Error("Failed to initialize bot", "error", err)
		os.Exit(1)
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT and SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Info("Received stop signal", "signal", sig)
		cancel()
	}()

	// Start healthcheck server
	mux := http.NewServeMux()
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		log.Info("Starting healthcheck server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Healthcheck server failed", "error", err)
		}
	}()

	sched := scheduler.New(log, func(taskCtx context.Context, hour, minute int) {
		b.SendReminders(taskCtx, hour, minute, time.Now())
	})

	go sched.Start(ctx)

	// Start bot polling
	updates, err := b.GetUpdatesChan()
	if err != nil {
		log.Error("Failed to get updates channel", "error", err)
		os.Exit(1)
	}

	log.Info("Bot successfully started and polling for updates")

	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down bot...")
			return
		case update := <-updates:
			b.HandleUpdate(update)
		}
	}
}
