package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/kv"
	"github.com/levmi/vet-notifications-go/internal/logger"
	"github.com/levmi/vet-notifications-go/internal/reminder"
	"github.com/levmi/vet-notifications-go/internal/storage"
)

// weightsPerPage is the number of weight records per page.
const weightsPerPage = 5

// TelegramSender is an interface for sending messages (for mocking in tests)
type TelegramSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// Bot represents the Telegram bot.
type Bot struct {
	api     TelegramSender
	logger  *slog.Logger
	storage storage.Storage
	kvStore kv.KVStore
}

// New creates a new Bot instance.
func New(token string, logger *slog.Logger, st storage.Storage, kvStore kv.KVStore) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	b := &Bot{
		api:     api,
		logger:  logger,
		storage: st,
		kvStore: kvStore,
	}

	if err := b.SetCommands(); err != nil {
		logger.Error("Failed to set bot commands", "error", err)
	}

	return b, nil
}

// NewWithSender creates a new bot with a custom sender (for tests)
func NewWithSender(sender TelegramSender, logger *slog.Logger, st storage.Storage, kvStore kv.KVStore) *Bot {
	return &Bot{
		api:     sender,
		logger:  logger,
		storage: st,
		kvStore: kvStore,
	}
}

// GetUpdatesChan returns a channel for receiving updates from Telegram.
func (b *Bot) GetUpdatesChan() (tgbotapi.UpdatesChannel, error) {
	api, ok := b.api.(*tgbotapi.BotAPI)
	if !ok {
		return nil, fmt.Errorf("underlying API is not tgbotapi.BotAPI")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	return api.GetUpdatesChan(u), nil
}

// SetCommands sets the bot menu commands.
func (b *Bot) SetCommands() error {
	commands := []tgbotapi.BotCommand{
		{Command: "today", Description: "Что нужно делать сегодня"},
		{Command: "weight", Description: "Записать вес: /weight [число]"},
		{Command: "add-pet", Description: "Добавить питомца: /add-pet [имя]"},
		{Command: "my-pets", Description: "Список питомцев"},
		{Command: "injection-site", Description: "Изменить стартовое место укола"},
		{Command: "start-date", Description: "Установить дату начала: /start-date ГГГГ-ММ-ДД"},
		{Command: "reminder-time", Description: "Установить время напоминания: /reminder-time ЧЧ:ММ"},
		{Command: "help", Description: "Справка по командам"},
	}

	cmdConfig := tgbotapi.NewSetMyCommands(commands...)
	_, err := b.api.Request(cmdConfig)
	return err
}

// HandleUpdate processes a single update from Telegram.
func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	traceLogger := logger.WithTraceID(b.logger)

	var userID int64
	var username string
	if update.Message != nil {
		userID = update.Message.From.ID
		username = update.Message.From.UserName
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
		username = update.CallbackQuery.From.UserName
	} else {
		return
	}

	if update.CallbackQuery != nil {
		traceLogger.Info("Received callback query",
			"userID", userID,
			"username", username,
			"data", update.CallbackQuery.Data,
		)
		b.handleCallbackQuery(traceLogger, update.CallbackQuery)
		return
	}

	traceLogger.Info("Received message",
		"userID", userID,
		"username", username,
		"text", update.Message.Text,
	)

	if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "start":
			b.handleStart(traceLogger, update.Message)
		case "help":
			b.handleHelp(traceLogger, update.Message)
		case "today":
			b.handleToday(traceLogger, update.Message)
		case "weight":
			b.handleWeight(traceLogger, update.Message)
		case "add-pet":
			b.handleAddPet(traceLogger, update.Message)
		case "my-pets":
			b.handleMyPets(traceLogger, update.Message)
		case "injection-site":
			b.handleInjectionSite(traceLogger, update.Message)
		case "start-date":
			b.handleStartDate(traceLogger, update.Message)
		case "reminder-time":
			b.handleReminderTime(traceLogger, update.Message)
		default:
			traceLogger.Warn("Unknown command", "command", update.Message.Command())
		}
	}
}

// SendReminders sends reminders to all pets whose reminder_time matches the given hour/minute.
func (b *Bot) SendReminders(ctx context.Context, hour, minute int, now time.Time) {
	traceLogger := logger.WithTraceID(b.logger)

	pets, err := b.storage.GetPetsForReminder(hour, minute)
	if err != nil {
		traceLogger.Error("Failed to get pets for reminder", "error", err, "hour", hour, "minute", minute)
		return
	}

	for _, pet := range pets {
		dayNumber := reminder.CalculateDayNumber(pet.StartDate, now)
		messageText := reminder.FormatMessage(dayNumber, now, pet.Name, pet.StartInjectionIndex)

		done := false
		isDone, err := b.storage.IsInjectionDone(pet.ID, now)
		if err != nil {
			traceLogger.Error("Failed to check if injection is done", "error", err)
		}
		done = isDone

		if done {
			messageText += "\n\n✅ **Укол уже сделан!**"
		}

		// Retry logic: 3 attempts with exponential backoff
		maxRetries := 3
		var sendErr error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			msg := tgbotapi.NewMessage(pet.UserID, messageText)
			msg.ParseMode = tgbotapi.ModeMarkdown

			if !done {
				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Сделано!", fmt.Sprintf("done_%d_%s", pet.ID, now.Format("2006-01-02"))),
					),
				)
				msg.ReplyMarkup = keyboard
			}

			_, sendErr = b.api.Send(msg)
			if sendErr == nil {
				traceLogger.Info("Reminder sent successfully", "userID", pet.UserID, "dayNumber", dayNumber, "petName", pet.Name)
				break
			}

			traceLogger.Warn("Failed to send reminder", "attempt", attempt, "userID", pet.UserID, "error", sendErr)

			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					traceLogger.Error("Context cancelled while retrying to send reminder", "userID", pet.UserID)
					return
				case <-time.After(time.Duration(attempt) * time.Second):
				}
			}
		}

		if sendErr != nil {
			traceLogger.Error("Failed to send reminder after all retries", "userID", pet.UserID, "error", sendErr)
		}
	}
}
