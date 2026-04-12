package bot

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/reminder"
	"github.com/levmi/vet-notifications-go/internal/sanitize"
)

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

	if strings.HasPrefix(data, "set_start_date:") {
		b.handleStartDateCallback(log, callbackQuery)
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

func (b *Bot) handleStartDateCallback(log *slog.Logger, callbackQuery *tgbotapi.CallbackQuery) {
	parts := strings.Split(callbackQuery.Data, ":")
	if len(parts) != 3 {
		log.Error("Invalid set_start_date callback data", "data", callbackQuery.Data)
		return
	}

	petID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse petID", "error", err)
		return
	}

	dateStr := parts[2]
	date, err := sanitize.DateString(dateStr)
	if err != nil {
		log.Error("Invalid date", "error", err, "date", dateStr)
		return
	}

	pet, err := b.storage.GetPet(petID)
	if err != nil || pet == nil || pet.UserID != callbackQuery.From.ID {
		log.Warn("Unauthorized pet access or not found", "petID", petID, "userID", callbackQuery.From.ID)
		return
	}

	if err := b.storage.SetStartDate(petID, date); err != nil {
		log.Error("Failed to set start date", "error", err)
		return
	}

	callback := tgbotapi.NewCallback(callbackQuery.ID, "Установлено!")
	if _, err := b.api.Request(callback); err != nil {
		log.Error("Failed to answer callback", "error", err)
	}

	if callbackQuery.Message != nil {
		newText := fmt.Sprintf("✅ Дата начала цикла для «%s» установлена: **%s**", pet.Name, date.Format("02.01.2006"))
		editMsg := tgbotapi.NewEditMessageText(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID, newText)
		editMsg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.api.Request(editMsg); err != nil {
			log.Error("Failed to edit message", "error", err)
		}
	}
}
