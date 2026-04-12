package kv

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestTiKVAddrs(t *testing.T) []string {
	addrs := os.Getenv("TEST_TIKV_PD_ADDRS")
	if addrs == "" {
		t.Skip("TEST_TIKV_PD_ADDRS not set, skipping TiKV tests")
	}
	var result []string
	for _, a := range strings.Split(addrs, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			result = append(result, a)
		}
	}
	return result
}

func TestTiKVStore_SetAndGetActivePet(t *testing.T) {
	addrs := getTestTiKVAddrs(t)

	store, err := NewTiKVStore(addrs)
	require.NoError(t, err)
	defer store.Close()

	userID := int64(123456789)

	// Should return 0 when not set
	petID, err := store.GetActivePet(userID)
	assert.NoError(t, err)
	assert.Equal(t, 0, petID)

	// Set active pet
	err = store.SetActivePet(userID, 42)
	assert.NoError(t, err)

	// Should return the set value
	petID, err = store.GetActivePet(userID)
	assert.NoError(t, err)
	assert.Equal(t, 42, petID)

	// Update active pet
	err = store.SetActivePet(userID, 99)
	assert.NoError(t, err)

	petID, err = store.GetActivePet(userID)
	assert.NoError(t, err)
	assert.Equal(t, 99, petID)
}
