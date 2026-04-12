package bot

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/reminder"
	"github.com/levmi/vet-notifications-go/internal/sanitize"
)

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
	helpText := "🐾 *Доступные команды:*\n\n" +
		"🔹 /start — Приветственное сообщение\n" +
		"🔹 /help — Этот список команд\n\n" +
		"🐈 *Управление питомцами:*\n" +
		"🔹 `/add-pet [имя]` — Добавить питомца\n" +
		"   _Пример:_ `/add-pet Мурка`\n" +
		"🔹 /my-pets — Список ваших питомцев и выбор активного\n\n" +
		"💉 *Уколы и цикл:*\n" +
		"🔹 /today — Узнать статус укола на сегодня\n" +
		"🔹 /injection-site — Выбрать место самого первого укола в цикле\n" +
		"🔹 `/start-date [дата]` — Задать дату начала цикла. Если вызвать без даты, бот предложит удобные кнопки (Сегодня, Вчера и т.д.)\n" +
		"   _Примеры:_ `/start-date 08.03.2026` или `/start-date 2026.03.08`\n" +
		"🔹 `/reminder-time [ЧЧ:ММ]` — Установить время напоминания (в UTC)\n" +
		"   _Пример:_ `/reminder-time 16:00`\n\n" +
		"⚖️ *Взвешивание:*\n" +
		"🔹 `/weight [число]` — Записать текущий вес\n" +
		"   _Пример:_ `/weight 3.5`\n" +
		"🔹 /weight — Посмотреть историю взвешивания с кнопками страниц\n"

	msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
	msg.ParseMode = tgbotapi.ModeMarkdown
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
			fmt.Sprintf("📅 Текущая дата начала цикла для «%s»: **%s**\n\nВыберите из вариантов или отправьте дату: /start-date ГГГГ-ММ-ДД\nНапример: /start-date 2026-03-08",
				pet.Name, pet.StartDate.Format("02.01.2006")))
		msg.ParseMode = tgbotapi.ModeMarkdown
		
		keyboard := buildStartDateKeyboard(pet.ID)
		msg.ReplyMarkup = keyboard

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
