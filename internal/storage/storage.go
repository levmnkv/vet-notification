package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Pet represents a pet belonging to a user.
type Pet struct {
	ID                  int
	UserID              int64
	Name                string
	StartInjectionIndex int
	StartDate           time.Time
	ReminderHour        int
	ReminderMinute      int
	CreatedAt           string
}

// WeightRecord represents a single weight entry.
type WeightRecord struct {
	ID        int
	Date      string
	Weight    float64
	CreatedAt string
}

// Storage defines the interface for data persistence.
type Storage interface {
	// Pets
	AddPet(userID int64, name string) (int, error)
	GetPets(userID int64) ([]Pet, error)
	GetPet(petID int) (*Pet, error)
	GetAllUserIDs() ([]int64, error)
	GetPetsForReminder(hour, minute int) ([]Pet, error)
	SetInjectionSiteIndex(petID int, index int) error
	SetStartDate(petID int, date time.Time) error
	SetReminderTime(petID int, hour, minute int) error

	// Weights (linked to pet_id)
	AddWeight(petID int, date time.Time, weight float64) error
	GetWeights(petID int, limit int) ([]WeightRecord, error)
	GetWeightsPage(petID int, limit, offset int) ([]WeightRecord, error)
	CountWeights(petID int) (int, error)

	// Injections (linked to pet_id)
	MarkInjectionDone(petID int, date time.Time) error
	IsInjectionDone(petID int, date time.Time) (bool, error)

	Close() error
}

// PostgresStorage is the PostgreSQL implementation of Storage.
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates and initializes a new PostgreSQL storage.
func NewPostgresStorage(databaseURL string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &PostgresStorage{db: db}
	if err := storage.init(); err != nil {
		return nil, fmt.Errorf("failed to init database: %w", err)
	}

	return storage, nil
}

// init creates the necessary tables if they don't exist.
func (s *PostgresStorage) init() error {
	queryPets := `
	CREATE TABLE IF NOT EXISTS pets (
		id SERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		name TEXT NOT NULL,
		start_injection_index INTEGER NOT NULL DEFAULT 0,
		start_date DATE NOT NULL DEFAULT CURRENT_DATE,
		reminder_time TIME NOT NULL DEFAULT '16:00',
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
	`
	if _, err := s.db.Exec(queryPets); err != nil {
		return fmt.Errorf("failed to create pets table: %w", err)
	}

	// Migration: add start_date and reminder_time if they don't exist
	migrations := []string{
		`ALTER TABLE pets ADD COLUMN IF NOT EXISTS start_date DATE NOT NULL DEFAULT CURRENT_DATE`,
		`ALTER TABLE pets ADD COLUMN IF NOT EXISTS reminder_time TIME NOT NULL DEFAULT '16:00'`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("failed to run migration: %w", err)
		}
	}

	queryWeights := `
	CREATE TABLE IF NOT EXISTS weights (
		id SERIAL PRIMARY KEY,
		pet_id INTEGER NOT NULL REFERENCES pets(id),
		date DATE NOT NULL,
		weight DOUBLE PRECISION NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);
	`
	if _, err := s.db.Exec(queryWeights); err != nil {
		return fmt.Errorf("failed to create weights table: %w", err)
	}

	queryInjections := `
	CREATE TABLE IF NOT EXISTS injections (
		id SERIAL PRIMARY KEY,
		pet_id INTEGER NOT NULL REFERENCES pets(id),
		date DATE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(pet_id, date)
	);
	`
	if _, err := s.db.Exec(queryInjections); err != nil {
		return fmt.Errorf("failed to create injections table: %w", err)
	}

	// Performance Indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_pets_user_id ON pets(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_pets_reminder_time ON pets(reminder_time);`,
		`CREATE INDEX IF NOT EXISTS idx_weights_pet_id ON weights(pet_id);`,
		`CREATE INDEX IF NOT EXISTS idx_injections_pet_id ON injections(pet_id);`,
	}
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx, err)
		}
	}

	return nil
}

// petColumns is the list of columns to scan for a Pet.
const petColumns = `id, user_id, name, start_injection_index, start_date, EXTRACT(HOUR FROM reminder_time)::int, EXTRACT(MINUTE FROM reminder_time)::int, created_at`

// scanPet scans a Pet from a row.
func scanPet(scanner interface{ Scan(dest ...any) error }) (*Pet, error) {
	var p Pet
	if err := scanner.Scan(&p.ID, &p.UserID, &p.Name, &p.StartInjectionIndex, &p.StartDate, &p.ReminderHour, &p.ReminderMinute, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// AddPet adds a new pet for the user and returns its ID.
func (s *PostgresStorage) AddPet(userID int64, name string) (int, error) {
	var petID int
	query := `INSERT INTO pets (user_id, name) VALUES ($1, $2) RETURNING id`
	err := s.db.QueryRow(query, userID, name).Scan(&petID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert pet: %w", err)
	}
	return petID, nil
}

// GetPets returns all pets for a given user.
func (s *PostgresStorage) GetPets(userID int64) ([]Pet, error) {
	query := fmt.Sprintf(`SELECT %s FROM pets WHERE user_id = $1 ORDER BY id`, petColumns)
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pets: %w", err)
	}
	defer rows.Close()

	var pets []Pet
	for rows.Next() {
		p, err := scanPet(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pet row: %w", err)
		}
		pets = append(pets, *p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pet rows: %w", err)
	}

	return pets, nil
}

// GetPet returns a single pet by ID.
func (s *PostgresStorage) GetPet(petID int) (*Pet, error) {
	query := fmt.Sprintf(`SELECT %s FROM pets WHERE id = $1`, petColumns)
	p, err := scanPet(s.db.QueryRow(query, petID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query pet: %w", err)
	}
	return p, nil
}

// GetAllUserIDs returns a list of all unique user IDs that have at least one pet.
func (s *PostgresStorage) GetAllUserIDs() ([]int64, error) {
	query := `SELECT DISTINCT user_id FROM pets`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all user IDs: %w", err)
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user ID rows: %w", err)
	}

	return userIDs, nil
}

// GetPetsForReminder returns all pets whose reminder_time matches the given hour and minute.
func (s *PostgresStorage) GetPetsForReminder(hour, minute int) ([]Pet, error) {
	timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
	query := fmt.Sprintf(`SELECT %s FROM pets WHERE reminder_time = $1`, petColumns)
	rows, err := s.db.Query(query, timeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query pets for reminder: %w", err)
	}
	defer rows.Close()

	var pets []Pet
	for rows.Next() {
		p, err := scanPet(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pet row: %w", err)
		}
		pets = append(pets, *p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reminder pet rows: %w", err)
	}

	return pets, nil
}

// SetInjectionSiteIndex updates the starting injection site index for a pet.
func (s *PostgresStorage) SetInjectionSiteIndex(petID int, index int) error {
	query := `UPDATE pets SET start_injection_index = $1 WHERE id = $2`
	_, err := s.db.Exec(query, index, petID)
	if err != nil {
		return fmt.Errorf("failed to update injection site index: %w", err)
	}
	return nil
}

// SetStartDate updates the start date for a pet.
func (s *PostgresStorage) SetStartDate(petID int, date time.Time) error {
	query := `UPDATE pets SET start_date = $1 WHERE id = $2`
	dateStr := date.Format("2006-01-02")
	_, err := s.db.Exec(query, dateStr, petID)
	if err != nil {
		return fmt.Errorf("failed to update start date: %w", err)
	}
	return nil
}

// SetReminderTime updates the reminder time for a pet.
func (s *PostgresStorage) SetReminderTime(petID int, hour, minute int) error {
	timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
	query := `UPDATE pets SET reminder_time = $1 WHERE id = $2`
	_, err := s.db.Exec(query, timeStr, petID)
	if err != nil {
		return fmt.Errorf("failed to update reminder time: %w", err)
	}
	return nil
}

// AddWeight adds a new weight record for a pet.
func (s *PostgresStorage) AddWeight(petID int, date time.Time, weight float64) error {
	query := `INSERT INTO weights (pet_id, date, weight) VALUES ($1, $2, $3)`
	dateStr := date.Format("2006-01-02")
	_, err := s.db.Exec(query, petID, dateStr, weight)
	if err != nil {
		return fmt.Errorf("failed to insert weight: %w", err)
	}
	return nil
}

// GetWeights retrieves the latest weight records for a pet up to the specified limit.
func (s *PostgresStorage) GetWeights(petID int, limit int) ([]WeightRecord, error) {
	query := `SELECT id, date, weight, created_at FROM weights WHERE pet_id = $1 ORDER BY date DESC, id DESC LIMIT $2`

	rows, err := s.db.Query(query, petID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query weights: %w", err)
	}
	defer rows.Close()

	var records []WeightRecord
	for rows.Next() {
		var r WeightRecord
		if err := rows.Scan(&r.ID, &r.Date, &r.Weight, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan weight row: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating weight rows: %w", err)
	}

	return records, nil
}

// GetWeightsPage retrieves weight records with limit and offset for pagination.
func (s *PostgresStorage) GetWeightsPage(petID int, limit, offset int) ([]WeightRecord, error) {
	query := `SELECT id, date, weight, created_at FROM weights WHERE pet_id = $1 ORDER BY date DESC, id DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(query, petID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query weights page: %w", err)
	}
	defer rows.Close()

	var records []WeightRecord
	for rows.Next() {
		var r WeightRecord
		if err := rows.Scan(&r.ID, &r.Date, &r.Weight, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan weight row: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating weight rows: %w", err)
	}

	return records, nil
}

// CountWeights returns the total number of weight records for a pet.
func (s *PostgresStorage) CountWeights(petID int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM weights WHERE pet_id = $1`, petID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count weights: %w", err)
	}
	return count, nil
}

// MarkInjectionDone records that an injection was given on the specified date for a pet.
func (s *PostgresStorage) MarkInjectionDone(petID int, date time.Time) error {
	query := `INSERT INTO injections (pet_id, date) VALUES ($1, $2) ON CONFLICT (pet_id, date) DO NOTHING`
	dateStr := date.Format("2006-01-02")
	_, err := s.db.Exec(query, petID, dateStr)
	if err != nil {
		return fmt.Errorf("failed to insert injection: %w", err)
	}
	return nil
}

// IsInjectionDone checks if an injection was recorded for the given date and pet.
func (s *PostgresStorage) IsInjectionDone(petID int, date time.Time) (bool, error) {
	query := `SELECT 1 FROM injections WHERE pet_id = $1 AND date = $2`
	dateStr := date.Format("2006-01-02")

	var dummy int
	err := s.db.QueryRow(query, petID, dateStr).Scan(&dummy)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check injection status: %w", err)
	}
	return true, nil
}

// Close closes the database connection.
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
