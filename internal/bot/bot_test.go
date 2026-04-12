package bot

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/levmi/vet-notifications-go/internal/storage"
	"github.com/stretchr/testify/mock"
)

// MockTelegramSender is a mock for TelegramSender
type MockTelegramSender struct {
	mock.Mock
}

func (m *MockTelegramSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockTelegramSender) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	args := m.Called(c)

	// Handle nil return
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tgbotapi.APIResponse), args.Error(1)
}

// MockStorage is a mock for storage.Storage
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) AddPet(userID int64, name string) (int, error) {
	args := m.Called(userID, name)
	return args.Int(0), args.Error(1)
}

func (m *MockStorage) GetPets(userID int64) ([]storage.Pet, error) {
	args := m.Called(userID)
	return args.Get(0).([]storage.Pet), args.Error(1)
}

func (m *MockStorage) GetPet(petID int) (*storage.Pet, error) {
	args := m.Called(petID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.Pet), args.Error(1)
}

func (m *MockStorage) GetAllUserIDs() ([]int64, error) {
	args := m.Called()
	return args.Get(0).([]int64), args.Error(1)
}

func (m *MockStorage) SetInjectionSiteIndex(petID int, index int) error {
	args := m.Called(petID, index)
	return args.Error(0)
}

func (m *MockStorage) AddWeight(petID int, date time.Time, weight float64) error {
	args := m.Called(petID, date, weight)
	return args.Error(0)
}

func (m *MockStorage) GetWeights(petID int, limit int) ([]storage.WeightRecord, error) {
	args := m.Called(petID, limit)
	return args.Get(0).([]storage.WeightRecord), args.Error(1)
}

func (m *MockStorage) MarkInjectionDone(petID int, date time.Time) error {
	args := m.Called(petID, date)
	return args.Error(0)
}

func (m *MockStorage) IsInjectionDone(petID int, date time.Time) (bool, error) {
	args := m.Called(petID, date)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorage) GetWeightsPage(petID int, limit, offset int) ([]storage.WeightRecord, error) {
	args := m.Called(petID, limit, offset)
	return args.Get(0).([]storage.WeightRecord), args.Error(1)
}

func (m *MockStorage) CountWeights(petID int) (int, error) {
	args := m.Called(petID)
	return args.Int(0), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockKVStore is a mock for kv.KVStore
type MockKVStore struct {
	mock.Mock
}

func (m *MockKVStore) SetActivePet(userID int64, petID int) error {
	args := m.Called(userID, petID)
	return args.Error(0)
}

func (m *MockKVStore) GetActivePet(userID int64) (int, error) {
	args := m.Called(userID)
	return args.Int(0), args.Error(1)
}

func (m *MockKVStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestSendReminders_Success(t *testing.T) {
	mockSender := new(MockTelegramSender)
	mockStorage := new(MockStorage)
	mockKV := new(MockKVStore)
	logger := slog.Default()

	bot := NewWithSender(mockSender, logger, time.Now(), mockStorage, mockKV)

	// Setup: user has active pet
	mockStorage.On("GetAllUserIDs").Return([]int64{111}, nil)
	mockKV.On("GetActivePet", int64(111)).Return(1, nil)
	mockStorage.On("GetPet", 1).Return(&storage.Pet{ID: 1, UserID: 111, Name: "Мурка", StartInjectionIndex: 0}, nil)
	mockStorage.On("IsInjectionDone", 1, mock.AnythingOfType("time.Time")).Return(false, nil)

	mockSender.On("Send", mock.AnythingOfType("tgbotapi.MessageConfig")).Return(tgbotapi.Message{}, nil).Once()

	bot.SendReminders(context.Background(), 0, time.Now())

	mockSender.AssertExpectations(t)
}

func TestSendReminders_RetryLogic(t *testing.T) {
	mockSender := new(MockTelegramSender)
	mockStorage := new(MockStorage)
	mockKV := new(MockKVStore)
	logger := slog.Default()

	bot := NewWithSender(mockSender, logger, time.Now(), mockStorage, mockKV)

	mockStorage.On("GetAllUserIDs").Return([]int64{111}, nil)
	mockKV.On("GetActivePet", int64(111)).Return(1, nil)
	mockStorage.On("GetPet", 1).Return(&storage.Pet{ID: 1, UserID: 111, Name: "Мурка", StartInjectionIndex: 0}, nil)
	mockStorage.On("IsInjectionDone", 1, mock.AnythingOfType("time.Time")).Return(false, nil)

	// Fail twice, succeed on third attempt
	mockSender.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("api error")).Twice()
	mockSender.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bot.SendReminders(ctx, 0, time.Now())

	mockSender.AssertExpectations(t)
}

func TestHandleStart(t *testing.T) {
	mockSender := new(MockTelegramSender)
	mockStorage := new(MockStorage)
	mockKV := new(MockKVStore)
	logger := slog.Default()

	bot := NewWithSender(mockSender, logger, time.Now(), mockStorage, mockKV)

	mockStorage.On("GetPets", int64(111)).Return([]storage.Pet{}, nil)

	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 111},
			From: &tgbotapi.User{ID: 111},
			Text: "/start",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
		},
	}

	mockSender.On("Send", mock.AnythingOfType("tgbotapi.MessageConfig")).Return(tgbotapi.Message{}, nil).Once()

	bot.HandleUpdate(update)

}
