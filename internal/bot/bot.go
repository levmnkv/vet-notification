package bot

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/kv"
	"github.com/levmi/vet-notifications-go/internal/logger"
	"github.com/levmi/vet-notifications-go/internal/reminder"
	"github.com/levmi/vet-notifications-go/internal/sanitize"
	"github.com/levmi/vet-notifications-go/internal/storage"
)

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

func (b *Bot) handleStart(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pets, err := b.storage.GetPets(userID)
	if err != nil {
		log.Error("Failed to get pets", "error", err, "userID", userID)
	}

	var text string
	if len(pets) == 0 {
		text = "Привет! Я бот для ветеринарных напоминаний. Я буду присылать напоминания об уколах каждый день.\n\n" +
			"Для начала добавьте питомца командой:\n/add-pet [имя]\n\nНапример: /add-pet Мурка"
	} else {
		petNames := make([]string, len(pets))
		for i, p := range pets {
			petNames[i] = p.Name
		}
		text = fmt.Sprintf("Привет! Ваши питомцы: %s\n\nВведите /help для списка команд.", strings.Join(petNames, ", "))
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send start message", "error", err, "userID", userID)
	} else {
		log.Info("Successfully replied to user", "userID", userID, "command", "start")
	}
}

func (b *Bot) handleHelp(log *slog.Logger, message *tgbotapi.Message) {
	helpText := "Доступные команды:\n" +
		"/start - Приветственное сообщение\n" +
		"/help - Список доступных команд\n" +
		"/add-pet [имя] - Добавить питомца\n" +
		"/my-pets - Список питомцев и выбор активного\n" +
		"/today - Узнать, что нужно делать сегодня\n" +
		"/weight [число] - Записать вес (например, /weight 3.5)\n" +
		"/weight - Посмотреть историю записей веса\n" +
		"/injection-site - Изменить стартовое место укола\n" +
		"/start-date ГГГГ-ММ-ДД - Установить дату начала цикла\n" +
		"/reminder-time ЧЧ:ММ - Установить время напоминания (UTC)"

	msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
	_, err := b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send help message", "error", err, "userID", message.From.ID)
	} else {
		log.Info("Successfully replied to user", "userID", message.From.ID, "command", "help")
	}
}

func (b *Bot) handleAddPet(log *slog.Logger, message *tgbotapi.Message) {
	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Укажите имя питомца. Например: /add-pet Мурка")
		_, _ = b.api.Send(msg)
		return
	}

	// Validate pet name
	cleanName, err := sanitize.PetName(args)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = b.api.Send(msg)
		return
	}

	petID, err := b.storage.AddPet(message.From.ID, cleanName)
	if err != nil {
		log.Error("Failed to add pet", "error", err, "userID", message.From.ID)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при добавлении питомца.")
		_, _ = b.api.Send(msg)
		return
	}

	// If this is the first pet, set as active
	pets, _ := b.storage.GetPets(message.From.ID)
	if len(pets) == 1 {
		if err := b.kvStore.SetActivePet(message.From.ID, petID); err != nil {
			log.Error("Failed to set active pet in KV", "error", err)
		}
	}

	// Send confirmation with injection site selection buttons
	text := fmt.Sprintf("✅ Питомец «%s» добавлен!\n\nВыберите стартовое место укола:", cleanName)
	keyboard := buildInjectionSiteKeyboard(petID, -1, "set_injection_site")
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send add-pet confirmation", "error", err)
	} else {
		log.Info("Pet added", "userID", message.From.ID, "petName", cleanName, "petID", petID)
	}
}

func (b *Bot) handleMyPets(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pets, err := b.storage.GetPets(userID)
	if err != nil {
		log.Error("Failed to get pets", "error", err, "userID", userID)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении списка питомцев.")
		_, _ = b.api.Send(msg)
		return
	}

	if len(pets) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас пока нет питомцев. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	activePetID, _ := b.kvStore.GetActivePet(userID)

	var sb strings.Builder
	sb.WriteString("🐾 Ваши питомцы:\n\n")
	for _, p := range pets {
		marker := ""
		if p.ID == activePetID {
			marker = " ✅ (активный)"
		}
		sb.WriteString(fmt.Sprintf("• %s (укол: %s, начало: %s, напоминание: %02d:%02d UTC)%s\n",
			p.Name,
			reminder.InjectionSites[p.StartInjectionIndex],
			p.StartDate.Format("02.01.2006"),
			p.ReminderHour, p.ReminderMinute,
			marker,
		))
	}
	sb.WriteString("\nВыберите активного питомца:")

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, p := range pets {
		label := p.Name
		if p.ID == activePetID {
			label = "✅ " + label
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("select_pet:%d", p.ID)),
		))
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	msg := tgbotapi.NewMessage(message.Chat.ID, sb.String())
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send my-pets", "error", err)
	}
}

func (b *Bot) handleInjectionSite(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pet, err := b.getActivePet(userID)
	if err != nil {
		log.Error("Failed to get active pet", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка.")
		_, _ = b.api.Send(msg)
		return
	}
	if pet == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активного питомца. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	text := fmt.Sprintf("💉 Текущее стартовое место укола для «%s»: **%s**\n\nВыберите новое:", pet.Name, reminder.InjectionSites[pet.StartInjectionIndex])
	keyboard := buildInjectionSiteKeyboard(pet.ID, pet.StartInjectionIndex, "change_injection_site")

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = keyboard
	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send injection-site message", "error", err)
	}
}

func (b *Bot) handleStartDate(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pet, err := b.getActivePet(userID)
	if err != nil {
		log.Error("Failed to get active pet", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка.")
		_, _ = b.api.Send(msg)
		return
	}
	if pet == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активного питомца. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("📅 Текущая дата начала цикла для «%s»: **%s**\n\nЧтобы изменить: /start-date ГГГГ-ММ-ДД\nНапример: /start-date 2026-03-08",
				pet.Name, pet.StartDate.Format("02.01.2006")))
		msg.ParseMode = tgbotapi.ModeMarkdown
		_, _ = b.api.Send(msg)
		return
	}

	date, err := sanitize.DateString(args)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = b.api.Send(msg)
		return
	}

	if err := b.storage.SetStartDate(pet.ID, date); err != nil {
		log.Error("Failed to set start date", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении даты.")
		_, _ = b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("✅ Дата начала цикла для «%s» установлена: **%s**", pet.Name, date.Format("02.01.2006")))
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, _ = b.api.Send(msg)
	log.Info("Start date updated", "userID", userID, "petID", pet.ID, "date", date.Format("2006-01-02"))
}

func (b *Bot) handleReminderTime(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pet, err := b.getActivePet(userID)
	if err != nil {
		log.Error("Failed to get active pet", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка.")
		_, _ = b.api.Send(msg)
		return
	}
	if pet == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активного питомца. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	args := strings.TrimSpace(message.CommandArguments())
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("⏰ Текущее время напоминания для «%s»: **%02d:%02d UTC**\n\nЧтобы изменить: /reminder-time ЧЧ:ММ\nНапример: /reminder-time 16:00",
				pet.Name, pet.ReminderHour, pet.ReminderMinute))
		msg.ParseMode = tgbotapi.ModeMarkdown
		_, _ = b.api.Send(msg)
		return
	}

	hour, minute, err := sanitize.TimeString(args)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = b.api.Send(msg)
		return
	}

	if err := b.storage.SetReminderTime(pet.ID, hour, minute); err != nil {
		log.Error("Failed to set reminder time", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении времени.")
		_, _ = b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("✅ Время напоминания для «%s» установлено: **%02d:%02d UTC**", pet.Name, hour, minute))
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, _ = b.api.Send(msg)
	log.Info("Reminder time updated", "userID", userID, "petID", pet.ID, "time", fmt.Sprintf("%02d:%02d", hour, minute))
}

// buildInjectionSiteKeyboard creates inline buttons for the 4 injection sites.
func buildInjectionSiteKeyboard(petID int, currentIndex int, prefix string) tgbotapi.InlineKeyboardMarkup {
	var row1, row2 []tgbotapi.InlineKeyboardButton
	for i, site := range reminder.InjectionSites {
		label := site
		if i == currentIndex {
			label += " ✓"
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("%s:%d:%d", prefix, petID, i))
		if i < 2 {
			row1 = append(row1, btn)
		} else {
			row2 = append(row2, btn)
		}
	}
	return tgbotapi.NewInlineKeyboardMarkup(row1, row2)
}

func (b *Bot) handleToday(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pet, err := b.getActivePet(userID)
	if err != nil {
		log.Error("Failed to get active pet", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка.")
		_, _ = b.api.Send(msg)
		return
	}
	if pet == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активного питомца. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	now := time.Now()
	dayNumber := reminder.CalculateDayNumber(pet.StartDate, now)
	messageText := reminder.FormatMessage(dayNumber, now, pet.Name, pet.StartInjectionIndex)

	done := false
	isDone, err := b.storage.IsInjectionDone(pet.ID, now)
	if err != nil {
		log.Error("Failed to check if injection is done", "error", err)
	}
	done = isDone

	if done {
		messageText += "\n\n✅ **Укол уже сделан!**"
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	msg.ParseMode = tgbotapi.ModeMarkdown

	if !done {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Сделано!", fmt.Sprintf("done_%d_%s", pet.ID, now.Format("2006-01-02"))),
			),
		)
		msg.ReplyMarkup = keyboard
	}

	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send today message", "error", err, "userID", userID)
	} else {
		log.Info("Successfully replied to user", "userID", userID, "command", "today")
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

// getActivePet returns the active pet for a user, or nil if none.
func (b *Bot) getActivePet(userID int64) (*storage.Pet, error) {
	petID, err := b.kvStore.GetActivePet(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active pet from KV: %w", err)
	}

	var pet *storage.Pet
	if petID != 0 {
		pet, err = b.storage.GetPet(petID)
		if err != nil {
			return nil, fmt.Errorf("failed to get pet: %w", err)
		}
	}

	if pet == nil || pet.UserID != userID {
		pets, err := b.storage.GetPets(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get pets: %w", err)
		}
		if len(pets) == 0 {
			return nil, nil
		}
		pet = &pets[0]
		_ = b.kvStore.SetActivePet(userID, pet.ID)
	}

	return pet, nil
}

// weightsPerPage is the number of weight records per page.
const weightsPerPage = 5

// handleWeight processes the /weight command.
func (b *Bot) handleWeight(log *slog.Logger, message *tgbotapi.Message) {
	userID := message.From.ID

	pet, err := b.getActivePet(userID)
	if err != nil {
		log.Error("Failed to get active pet", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка.")
		_, _ = b.api.Send(msg)
		return
	}
	if pet == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас нет активного питомца. Добавьте командой /add-pet [имя]")
		_, _ = b.api.Send(msg)
		return
	}

	args := strings.TrimSpace(message.CommandArguments())

	if args == "" {
		text, keyboard, err := b.buildWeightPage(pet.ID, pet.Name, 0)
		if err != nil {
			log.Error("Failed to build weight page", "error", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось получить историю веса.")
			_, _ = b.api.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		if keyboard != nil {
			msg.ReplyMarkup = keyboard
		}
		_, err = b.api.Send(msg)
		if err != nil {
			log.Error("Failed to send weight history", "error", err)
		} else {
			log.Info("Successfully replied to user with weight history", "userID", userID)
		}
		return
	}

	// Validate weight
	weight, err := sanitize.Weight(args)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = b.api.Send(msg)
		return
	}

	now := time.Now()
	if err := b.storage.AddWeight(pet.ID, now, weight); err != nil {
		log.Error("Failed to save weight", "error", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении веса.")
		_, _ = b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ Вес **%.2f кг** для **%s** успешно записан на %s.", weight, pet.Name, now.Format("02.01.2006")))
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err = b.api.Send(msg)
	if err != nil {
		log.Error("Failed to send weight confirmation", "error", err)
	} else {
		log.Info("Successfully replied to user with weight confirmation", "userID", userID)
	}
}

// buildWeightPage builds the text and keyboard for a given page of weights.
func (b *Bot) buildWeightPage(petID int, petName string, page int) (string, *tgbotapi.InlineKeyboardMarkup, error) {
	totalCount, err := b.storage.CountWeights(petID)
	if err != nil {
		return "", nil, err
	}

	if totalCount == 0 {
		return fmt.Sprintf("История взвешиваний для %s пуста.", petName), nil, nil
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(weightsPerPage)))
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	offset := page * weightsPerPage
	records, err := b.storage.GetWeightsPage(petID, weightsPerPage, offset)
	if err != nil {
		return "", nil, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 История веса %s (стр. %d/%d):\n", petName, page+1, totalPages))
	for _, r := range records {
		sb.WriteString(fmt.Sprintf("• %s: %.2f кг\n", r.Date, r.Weight))
	}

	var keyboard *tgbotapi.InlineKeyboardMarkup
	if totalPages > 1 {
		kb := buildWeightKeyboard(petID, page, totalPages)
		keyboard = &kb
	}

	return sb.String(), keyboard, nil
}

// buildWeightKeyboard builds the inline pagination keyboard.
func buildWeightKeyboard(petID int, currentPage, totalPages int) tgbotapi.InlineKeyboardMarkup {
	var buttons []tgbotapi.InlineKeyboardButton

	if currentPage > 0 {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("⏮", fmt.Sprintf("weight_page_%d_%d", petID, 0)))
	} else {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("⏮", "weight_noop"))
	}

	if currentPage > 0 {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("◀️", fmt.Sprintf("weight_page_%d_%d", petID, currentPage-1)))
	} else {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("◀️", "weight_noop"))
	}

	if currentPage < totalPages-1 {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("▶️", fmt.Sprintf("weight_page_%d_%d", petID, currentPage+1)))
	} else {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("▶️", "weight_noop"))
	}

	if currentPage < totalPages-1 {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("⏭", fmt.Sprintf("weight_page_%d_%d", petID, totalPages-1)))
	} else {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData("⏭", "weight_noop"))
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttons...),
	)
}

// handleCallbackQuery processes inline keyboard actions
func (b *Bot) handleCallbackQuery(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	data := callbackQuery.Data

	if data == "weight_noop" {
		callback := tgbotapi.NewCallback(callbackQuery.ID, "")
		if _, err := b.api.Request(callback); err != nil {
			log.Error("Failed to answer noop callback", "error", err)
		}
		return
	}

	if strings.HasPrefix(data, "weight_page_") {
		b.handleWeightPageCallback(log, callbackQuery)
		return
	}

	if strings.HasPrefix(data, "set_injection_site:") {
		b.handleSetInjectionSiteCallback(log, callbackQuery)
		return
	}

	if strings.HasPrefix(data, "change_injection_site:") {
		b.handleChangeInjectionSiteCallback(log, callbackQuery)
		return
	}

	if strings.HasPrefix(data, "select_pet:") {
		b.handleSelectPetCallback(log, callbackQuery)
		return
	}

	if strings.HasPrefix(data, "done_") {
		b.handleDoneCallback(log, callbackQuery)
		return
	}
}

func (b *Bot) handleWeightPageCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	trimmed := strings.TrimPrefix(callbackQuery.Data, "weight_page_")
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) != 2 {
		log.Error("Invalid weight page callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Error("Failed to parse petID from weight page callback", "data", callbackQuery.Data, "error", err)
		return
	}

	page, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse page from weight page callback", "data", callbackQuery.Data, "error", err)
		return
	}

	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer weight page callback", "error", err)
	}

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != callbackQuery.From.ID {
		log.Warn("Unauthorized pet access or not found for weight page", "petID", petID, "userID", callbackQuery.From.ID)
		return
	}

	text, keyboard, err := b.buildWeightPage(petID, pet.Name, page)
	if err != nil {
		log.Error("Failed to build weight page", "error", err, "page", page)
		return
	}

	if callbackQuery.Message != nil {
		editMsg := tgbotapi.NewEditMessageText(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			text,
		)
		if keyboard != nil {
			editMsg.ReplyMarkup = keyboard
		}
		if _, err := b.api.Request(editMsg); err != nil {
			log.Error("Failed to edit weight page message", "error", err)
		}
	}
}

func (b *Bot) handleSetInjectionSiteCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	parts := strings.Split(callbackQuery.Data, ":")
	if len(parts) != 3 {
		log.Error("Invalid set_injection_site callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse petID", "error", err)
		return
	}

	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 0 || index >= len(reminder.InjectionSites) {
		log.Error("Invalid injection site index", "error", err, "index", parts[2])
		return
	}

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != callbackQuery.From.ID {
		log.Warn("Unauthorized pet access or not found", "petID", petID, "userID", callbackQuery.From.ID)
		return
	}

	if err := b.storage.SetInjectionSiteIndex(petID, index); err != nil {
		log.Error("Failed to set injection site index", "error", err)
		return
	}

	petName := pet.Name

	callback := tgbotapi.NewCallback(callbackQuery.ID, "Установлено!")
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer callback", "error", err)
	}

	if callbackQuery.Message != nil {
		newText := fmt.Sprintf("✅ Стартовое место укола для %s: %s", petName, reminder.InjectionSites[index])
		editMsg := tgbotapi.NewEditMessageText(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID, newText)
		if _, err := b.api.Request(editMsg); err != nil {
			log.Error("Failed to edit message", "error", err)
		}
	}
}

func (b *Bot) handleChangeInjectionSiteCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	parts := strings.Split(callbackQuery.Data, ":")
	if len(parts) != 3 {
		log.Error("Invalid change_injection_site callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse petID", "error", err)
		return
	}

	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 0 || index >= len(reminder.InjectionSites) {
		log.Error("Invalid injection site index", "error", err, "index", parts[2])
		return
	}

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != callbackQuery.From.ID {
		log.Warn("Unauthorized pet access or not found", "petID", petID, "userID", callbackQuery.From.ID)
		return
	}

	if err := b.storage.SetInjectionSiteIndex(petID, index); err != nil {
		log.Error("Failed to set injection site index", "error", err)
		return
	}

	petName := pet.Name

	callback := tgbotapi.NewCallback(callbackQuery.ID, "Изменено!")
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer callback", "error", err)
	}

	if callbackQuery.Message != nil {
		newText := fmt.Sprintf("✅ Стартовое место укола для %s изменено: %s", petName, reminder.InjectionSites[index])
		editMsg := tgbotapi.NewEditMessageText(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID, newText)
		if _, err := b.api.Request(editMsg); err != nil {
			log.Error("Failed to edit message", "error", err)
		}
	}
}

func (b *Bot) handleSelectPetCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	parts := strings.Split(callbackQuery.Data, ":")
	if len(parts) != 2 {
		log.Error("Invalid select_pet callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse petID", "error", err)
		return
	}

	userID := callbackQuery.From.ID

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != userID {
		log.Warn("Unauthorized pet access or not found", "petID", petID, "userID", userID)
		return
	}

	if err := b.kvStore.SetActivePet(userID, petID); err != nil {
		log.Error("Failed to set active pet", "error", err)
		return
	}

	petName := pet.Name

	callback := tgbotapi.NewCallback(callbackQuery.ID, fmt.Sprintf("Активный: %s", petName))
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer callback", "error", err)
	}

	if callbackQuery.Message != nil {
		newText := fmt.Sprintf("✅ Активный питомец: **%s**", petName)
		editMsg := tgbotapi.NewEditMessageText(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID, newText)
		editMsg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.api.Request(editMsg); err != nil {
			log.Error("Failed to edit message", "error", err)
		}
	}

	log.Info("Active pet changed", "userID", userID, "petID", petID, "petName", petName)
}

func (b *Bot) handleDoneCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	trimmed := strings.TrimPrefix(callbackQuery.Data, "done_")
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) != 2 {
		log.Error("Invalid done callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Error("Failed to parse petID from done callback", "error", err)
		return
	}

	parsedDate, err := time.Parse("2006-01-02", parts[1])
	if err != nil {
		log.Error("Failed to parse date from done callback", "data", callbackQuery.Data, "error", err)
		return
	}

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != callbackQuery.From.ID {
		log.Warn("Unauthorized pet access or not found", "petID", petID, "userID", callbackQuery.From.ID)
		return
	}

	if err := b.storage.MarkInjectionDone(petID, parsedDate); err != nil {
		log.Error("Failed to mark injection done", "error", err, "date", parsedDate)
	}

	callback := tgbotapi.NewCallback(callbackQuery.ID, "Отмечено!")
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer callback query", "error", err)
	}

	if callbackQuery.Message != nil {
		originalText := callbackQuery.Message.Text
		if !strings.HasSuffix(originalText, "✅ Укол сделан!") && !strings.HasSuffix(originalText, "✅ Укол уже сделан!") {
			newText := originalText + "\n\n✅ **Укол уже сделан!**"
			editMsg := tgbotapi.NewEditMessageText(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID, newText)
			editMsg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := b.api.Request(editMsg); err != nil {
				log.Error("Failed to edit message for callback", "error", err)
			}
		}
	}
}
