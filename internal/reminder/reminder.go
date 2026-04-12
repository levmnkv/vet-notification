package reminder

import (
	"fmt"
	"time"
)

var InjectionSites = [...]string{
	"Левая паховая",
	"Левая лопатка",
	"Правая лопатка",
	"Правая паховая",
}

// GetInjectionSite returns the injection site for a given day number and starting index.
func GetInjectionSite(dayNumber int, startIndex int) string {
	index := (dayNumber + startIndex) % len(InjectionSites)
	if index < 0 {
		index += len(InjectionSites)
	}
	return InjectionSites[index]
}

// ShouldWeigh returns true if the pet should be weighed on this day.
func ShouldWeigh(dayNumber int) bool {
	return dayNumber%3 == 0
}

// FormatMessage formats the reminder message with pet name and injection site.
func FormatMessage(dayNumber int, date time.Time, petName string, startIndex int) string {
	site := GetInjectionSite(dayNumber, startIndex)
	weigh := ShouldWeigh(dayNumber)

	dateStr := date.Format("02.01.2006")

	msg := fmt.Sprintf("Напоминание об уколе **%s**! (%s)\nМесто укола сегодня: **%s**", petName, dateStr, site)
	if weigh {
		msg += fmt.Sprintf("\n\n⚠️ Нужно взвесить **%s**!", petName)
	}

	return msg
}

// CalculateDayNumber calculates the number of days between the start date and the current date.
func CalculateDayNumber(startDate, currentDate time.Time) int {
	start := startDate.Truncate(24 * time.Hour)
	current := currentDate.Truncate(24 * time.Hour)

	duration := current.Sub(start)
	return int(duration.Hours() / 24)
}
