package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDatabaseURL(t *testing.T) string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping PostgreSQL tests")
	}
	return url
}

func prepareTestDB(t *testing.T) *PostgresStorage {
	url := getTestDatabaseURL(t)

	store, err := NewPostgresStorage(url)
	require.NoError(t, err)

	// Clean up tables for test isolation
	_, _ = store.db.Exec("DELETE FROM injections")
	_, _ = store.db.Exec("DELETE FROM weights")
	_, _ = store.db.Exec("DELETE FROM pets")

	return store
}

func TestPostgresStorage_AddPetAndGetPets(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID1, err := store.AddPet(111, "Мурка")
	assert.NoError(t, err)
	assert.Greater(t, petID1, 0)

	petID2, err := store.AddPet(111, "Барсик")
	assert.NoError(t, err)
	assert.Greater(t, petID2, petID1)

	pets, err := store.GetPets(111)
	assert.NoError(t, err)
	assert.Len(t, pets, 2)
	assert.Equal(t, "Мурка", pets[0].Name)
	assert.Equal(t, "Барсик", pets[1].Name)
	assert.Equal(t, int64(111), pets[0].UserID)
}

func TestPostgresStorage_GetPet(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID, err := store.AddPet(111, "Мурка")
	require.NoError(t, err)

	pet, err := store.GetPet(petID)
	assert.NoError(t, err)
	assert.NotNil(t, pet)
	assert.Equal(t, "Мурка", pet.Name)
	assert.Equal(t, 0, pet.StartInjectionIndex)

	// Non-existent pet
	pet, err = store.GetPet(99999)
	assert.NoError(t, err)
	assert.Nil(t, pet)
}

func TestPostgresStorage_SetInjectionSiteIndex(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID, err := store.AddPet(111, "Мурка")
	require.NoError(t, err)

	err = store.SetInjectionSiteIndex(petID, 2)
	assert.NoError(t, err)

	pet, err := store.GetPet(petID)
	assert.NoError(t, err)
	assert.Equal(t, 2, pet.StartInjectionIndex)
}

func TestPostgresStorage_AddAndGetWeights(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID, err := store.AddPet(111, "Мурка")
	require.NoError(t, err)

	date1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	err = store.AddWeight(petID, date1, 3.5)
	assert.NoError(t, err)

	date2 := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	err = store.AddWeight(petID, date2, 3.6)
	assert.NoError(t, err)

	records, err := store.GetWeights(petID, 5)
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	// Latest should be first
	assert.Equal(t, 3.6, records[0].Weight)
	assert.Equal(t, 3.5, records[1].Weight)
}

func TestPostgresStorage_CountWeights(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID, err := store.AddPet(111, "Мурка")
	require.NoError(t, err)

	count, err := store.CountWeights(petID)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	date := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	_ = store.AddWeight(petID, date, 3.5)

	count, err = store.CountWeights(petID)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPostgresStorage_Injections(t *testing.T) {
	store := prepareTestDB(t)
	defer store.Close()

	petID, err := store.AddPet(111, "Мурка")
	require.NoError(t, err)

	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	// Should not be done initially
	done, err := store.IsInjectionDone(petID, date)
	assert.NoError(t, err)
	assert.False(t, done)

	// Mark done
	err = store.MarkInjectionDone(petID, date)
	assert.NoError(t, err)

	// Should be done now
	done, err = store.IsInjectionDone(petID, date)
	assert.NoError(t, err)
	assert.True(t, done)

	// Marking again should not error (ON CONFLICT DO NOTHING)
	err = store.MarkInjectionDone(petID, date)
	assert.NoError(t, err)
}
