package bot

import (
	"fmt"
	"math"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/reminder"
	"github.com/levmi/vet-notifications-go/internal/storage"
)

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

func buildStartDateKeyboard(petID int) tgbotapi.InlineKeyboardMarkup {
	now := time.Now()
	
	btnToday := tgbotapi.NewInlineKeyboardButtonData("Сегодня", fmt.Sprintf("set_start_date:%d:%s", petID, now.Format("2006-01-02")))
	btnYesterday := tgbotapi.NewInlineKeyboardButtonData("Вчера", fmt.Sprintf("set_start_date:%d:%s", petID, now.AddDate(0, 0, -1).Format("2006-01-02")))
	btn2DaysAgo := tgbotapi.NewInlineKeyboardButtonData("Позавчера", fmt.Sprintf("set_start_date:%d:%s", petID, now.AddDate(0, 0, -2).Format("2006-01-02")))
	btnTomorrow := tgbotapi.NewInlineKeyboardButtonData("Завтра", fmt.Sprintf("set_start_date:%d:%s", petID, now.AddDate(0, 0, 1).Format("2006-01-02")))

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btnToday, btnYesterday),
		tgbotapi.NewInlineKeyboardRow(btn2DaysAgo, btnTomorrow),
	)
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

	// Calculate total pages based on weightsPerPage, which is defined in bot.go or handlers.go
	// To avoid circular dependency within package, we redeclare const here or access it.
	// Since weightsPerPage was 5, let's just make it accessible
	const localWeightsPerPage = 5

	totalPages := int(math.Ceil(float64(totalCount) / float64(localWeightsPerPage)))
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	offset := page * localWeightsPerPage
	records, err := b.storage.GetWeightsPage(petID, localWeightsPerPage, offset)
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
