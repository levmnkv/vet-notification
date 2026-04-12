package reminder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetInjectionSite_DefaultStart(t *testing.T) {
	tests := []struct {
		dayNumber int
		expected  string
	}{
		{0, "Левая паховая"},
		{1, "Левая лопатка"},
		{2, "Правая лопатка"},
		{3, "Правая паховая"},
		{4, "Левая паховая"},
		{5, "Левая лопатка"},
		{6, "Правая лопатка"},
		{7, "Правая паховая"},
		{8, "Левая паховая"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.dayNumber)), func(t *testing.T) {
			actual := GetInjectionSite(tt.dayNumber, 0) // startIndex = 0
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestGetInjectionSite_CustomStart(t *testing.T) {
	// startIndex = 1 means day 0 starts at "Левая лопатка"
	assert.Equal(t, "Левая лопатка", GetInjectionSite(0, 1))
	assert.Equal(t, "Правая лопатка", GetInjectionSite(1, 1))
	assert.Equal(t, "Правая паховая", GetInjectionSite(2, 1))
	assert.Equal(t, "Левая паховая", GetInjectionSite(3, 1))

	// startIndex = 3 means day 0 starts at "Правая паховая"
	assert.Equal(t, "Правая паховая", GetInjectionSite(0, 3))
	assert.Equal(t, "Левая паховая", GetInjectionSite(1, 3))
}

func TestShouldWeigh(t *testing.T) {
	tests := []struct {
		dayNumber int
		expected  bool
	}{
		{0, true},
		{1, false},
		{2, false},
		{3, true},
		{4, false},
		{5, false},
		{6, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.dayNumber)), func(t *testing.T) {
			actual := ShouldWeigh(tt.dayNumber)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestFormatMessage(t *testing.T) {
	testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	msgDay0 := FormatMessage(0, testDate, "Мурка", 0)
	assert.Contains(t, msgDay0, "01.01.2024")
	assert.Contains(t, msgDay0, "Левая паховая")
	assert.Contains(t, msgDay0, "Нужно взвесить **Мурка**")
	assert.Contains(t, msgDay0, "Мурка")

	msgDay1 := FormatMessage(1, testDate, "Барсик", 0)
	assert.Contains(t, msgDay1, "Левая лопатка")
	assert.NotContains(t, msgDay1, "Нужно взвесить")
	assert.Contains(t, msgDay1, "Барсик")

	// Custom start index
	msgDay0Custom := FormatMessage(0, testDate, "Мурка", 2)
	assert.Contains(t, msgDay0Custom, "Правая лопатка")
}

func TestCalculateDayNumber(t *testing.T) {
	startDate := time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC)

	currentDate1 := time.Date(2026, 3, 8, 16, 0, 0, 0, time.UTC) // Day 0
	assert.Equal(t, 0, CalculateDayNumber(startDate, currentDate1))

	currentDate2 := time.Date(2026, 3, 9, 16, 0, 0, 0, time.UTC) // Day 1
	assert.Equal(t, 1, CalculateDayNumber(startDate, currentDate2))

	currentDate3 := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC) // Day 10
	assert.Equal(t, 10, CalculateDayNumber(startDate, currentDate3))

	currentDate4 := time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC) // Day -1
	assert.Equal(t, -1, CalculateDayNumber(startDate, currentDate4))
}
