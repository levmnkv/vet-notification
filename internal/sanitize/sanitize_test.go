package sanitize

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPetName_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Мурка", "Мурка"},
		{"Барсик", "Барсик"},
		{"  Мурка  ", "Мурка"},
		{"My Cat", "My Cat"},
		{"Кот-1", "Кот-1"},
		{"кошка_2", "кошка_2"},
		{"Пушистик 3", "Пушистик 3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := PetName(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPetName_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"spaces only", "   "},
		{"too long", strings.Repeat("а", 51)},
		{"sql injection semicolon", "Мурка; DROP TABLE pets;"},
		{"sql injection quote", "Мурка' OR '1'='1"},
		{"sql injection double dash", "Мурка--"},
		{"sql keyword DROP", "DROP"},
		{"sql keyword SELECT", "SELECT * FROM pets"},
		{"sql keyword UNION", "UNION ALL"},
		{"special chars", "Мурка<script>"},
		{"slash", "кот/собака"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PetName(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestPetName_MaxLength(t *testing.T) {
	// Exactly 50 characters should pass
	name50 := strings.Repeat("а", 50)
	result, err := PetName(name50)
	assert.NoError(t, err)
	assert.Equal(t, name50, result)

	// 51 characters should fail
	name51 := strings.Repeat("а", 51)
	_, err = PetName(name51)
	assert.Error(t, err)
}

func TestDateString_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{"2026-03-08", time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
		{"08.03.2026", time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
		{"2026.03.08", time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)},
		{"2020-01-01", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"2100-12-31", time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"  2026-06-15  ", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := DateString(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDateString_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"bad format", "08-03-2026"},
		{"too early", "2019-12-31"},
		{"too late", "2101-01-01"},
		{"not a date", "hello"},
		{"sql injection", "2026-03-08'; DROP TABLE pets; --"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DateString(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestTimeString_Valid(t *testing.T) {
	tests := []struct {
		input  string
		hour   int
		minute int
	}{
		{"16:00", 16, 0},
		{"0:00", 0, 0},
		{"23:59", 23, 59},
		{"  09:30  ", 9, 30},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, err := TimeString(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.hour, h)
			assert.Equal(t, tt.minute, m)
		})
	}
}

func TestTimeString_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"bad format", "1600"},
		{"hour too big", "24:00"},
		{"minute too big", "16:60"},
		{"negative hour", "-1:00"},
		{"not a time", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := TimeString(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestWeight_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"3.5", 3.5},
		{"3,5", 3.5},
		{"0.01", 0.01},
		{"200", 200},
		{"  4.2  ", 4.2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := Weight(tt.input)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestWeight_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"not a number", "abc"},
		{"too small", "0"},
		{"too small 2", "0.001"},
		{"too large", "200.1"},
		{"negative", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Weight(tt.input)
			assert.Error(t, err)
		})
	}
}
