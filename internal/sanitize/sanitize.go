package sanitize

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxPetNameLength = 50

// petNameAllowed matches letters (any script), digits, spaces, hyphens, and underscores.
var petNameAllowed = regexp.MustCompile(`^[\p{L}\p{N}\s\-_]+$`)

// dangerousPatterns contains substrings that should never appear in user input destined for the DB.
var dangerousPatterns = []string{
	"--", ";", "'", "\"", "/*", "*/", "\\",
	"DROP", "DELETE", "INSERT", "UPDATE", "SELECT",
	"ALTER", "CREATE", "EXEC", "UNION",
}

// PetName validates and sanitizes a pet name from user input.
// Returns the trimmed name or an error if it fails validation.
func PetName(name string) (string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return "", fmt.Errorf("имя питомца не может быть пустым")
	}

	if utf8.RuneCountInString(name) > maxPetNameLength {
		return "", fmt.Errorf("имя питомца слишком длинное (максимум %d символов)", maxPetNameLength)
	}

	if !petNameAllowed.MatchString(name) {
		return "", fmt.Errorf("имя питомца содержит недопустимые символы (разрешены буквы, цифры, пробелы и дефисы)")
	}

	upper := strings.ToUpper(name)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(upper, pattern) {
			return "", fmt.Errorf("имя питомца содержит недопустимую последовательность символов")
		}
	}

	return name, nil
}

// DateString parses and validates a date string in YYYY-MM-DD format.
// The date must be between 2020-01-01 and 2100-12-31 inclusive.
func DateString(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"2006.01.02",
	}

	var parsed time.Time
	var err error
	for _, format := range formats {
		parsed, err = time.Parse(format, s)
		if err == nil {
			break
		}
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("неверный формат даты. Используйте: ГГГГ-ММ-ДД, ДД.ММ.ГГГГ или ГГГГ.ММ.ДД")
	}

	minDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)

	if parsed.Before(minDate) || parsed.After(maxDate) {
		return time.Time{}, fmt.Errorf("дата должна быть между 2020-01-01 и 2100-12-31")
	}

	return parsed, nil
}

// TimeString parses and validates a time string in HH:MM format.
// Returns the hour (0-23) and minute (0-59).
func TimeString(s string) (int, int, error) {
	s = strings.TrimSpace(s)

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("неверный формат времени, используйте ЧЧ:ММ (например, 16:00)")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("часы должны быть от 0 до 23")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("минуты должны быть от 0 до 59")
	}

	return hour, minute, nil
}

// Weight parses and validates a weight value from user input.
// Accepts comma or dot as decimal separator. Range: 0.01 — 200 kg.
func Weight(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")

	w, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("неверный формат веса, используйте число (например, 3.5)")
	}

	if w < 0.01 || w > 200 {
		return 0, fmt.Errorf("вес должен быть от 0.01 до 200 кг")
	}

	return w, nil
}
