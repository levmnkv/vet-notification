package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetNextTargetTime(t *testing.T) {
	// Target time: 16:00 UTC
	targetHour := 16
	targetMinute := 0

	// Case 1: Before target time on the same day
	nowBefore := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	targetBefore := GetNextTargetTime(nowBefore, targetHour, targetMinute)
	assert.Equal(t, time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC), targetBefore)

	// Case 2: After target time on the same day (should be next day 16:00)
	nowAfter := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
	targetAfter := GetNextTargetTime(nowAfter, targetHour, targetMinute)
	assert.Equal(t, time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC), targetAfter)

	// Case 3: Exactly at target time (should be next day)
	nowExact := time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
	targetExact := GetNextTargetTime(nowExact, targetHour, targetMinute)
	assert.Equal(t, time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC), targetExact)
}
