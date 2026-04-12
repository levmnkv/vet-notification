package kv

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tikv/client-go/v2/rawkv"
)

// KVStore defines the interface for key-value storage of active pet selection.
type KVStore interface {
	SetActivePet(userID int64, petID int) error
	GetActivePet(userID int64) (int, error) // returns 0 if not set
	Close() error
}

// TiKVStore is the TiKV implementation of KVStore.
type TiKVStore struct {
	client *rawkv.Client
}

// NewTiKVStore creates a new TiKV store connected to the given PD addresses.
func NewTiKVStore(pdAddrs []string) (*TiKVStore, error) {
	client, err := rawkv.NewClientWithOpts(context.Background(), pdAddrs)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TiKV: %w", err)
	}
	return &TiKVStore{client: client}, nil
}

func activePetKey(userID int64) []byte {
	return []byte(fmt.Sprintf("active_pet:%d", userID))
}

// SetActivePet stores the active pet ID for a user.
func (s *TiKVStore) SetActivePet(userID int64, petID int) error {
	key := activePetKey(userID)
	value := []byte(strconv.Itoa(petID))
	err := s.client.Put(context.Background(), key, value)
	if err != nil {
		return fmt.Errorf("failed to set active pet in TiKV: %w", err)
	}
	return nil
}

// GetActivePet retrieves the active pet ID for a user. Returns 0 if not set.
func (s *TiKVStore) GetActivePet(userID int64) (int, error) {
	key := activePetKey(userID)
	value, err := s.client.Get(context.Background(), key)
	if err != nil {
		return 0, fmt.Errorf("failed to get active pet from TiKV: %w", err)
	}
	if value == nil || len(value) == 0 {
		return 0, nil
	}
	petID, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, fmt.Errorf("failed to parse active pet ID from TiKV: %w", err)
	}
	return petID, nil
}

// Close closes the TiKV client connection.
func (s *TiKVStore) Close() error {
	return s.client.Close()
}
