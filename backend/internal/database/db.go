package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection
func NewDB(dataDir string) (*DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "cracked_dev_meter.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &DB{db}

	// Run migrations
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return database, nil
}

// migrate creates the necessary tables
func (db *DB) migrate() error {
	queries := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT,
			ip_address TEXT NOT NULL,
			user_agent TEXT,
			is_paid BOOLEAN DEFAULT FALSE,
			stripe_customer_id TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		// Request logs table
		`CREATE TABLE IF NOT EXISTS request_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			method TEXT NOT NULL,
			user_agent TEXT,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Payments table
		`CREATE TABLE IF NOT EXISTS payments (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			stripe_payment_id TEXT NOT NULL,
			amount INTEGER NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Leaderboard tables
		`CREATE TABLE IF NOT EXISTS developer_analyses (
			id TEXT PRIMARY KEY,
			developer_hash TEXT NOT NULL UNIQUE, -- Anonymized developer identifier
			input_type TEXT NOT NULL, -- 'github', 'x', 'combined'
			input_value TEXT NOT NULL, -- The actual input (for display if allowed)
			score REAL NOT NULL,
			confidence REAL NOT NULL,
			posterior REAL NOT NULL,
			breakdown TEXT, -- JSON breakdown of categories
			github_username TEXT,
			x_username TEXT,
			ip_address TEXT NOT NULL,
			user_agent TEXT,
			is_public BOOLEAN DEFAULT FALSE, -- Whether to show on public leaderboard
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS leaderboard_entries (
			id TEXT PRIMARY KEY,
			developer_hash TEXT NOT NULL,
			period TEXT NOT NULL, -- 'daily', 'weekly', 'monthly', 'all_time'
			period_start DATE NOT NULL,
			period_end DATE NOT NULL,
			rank INTEGER NOT NULL,
			score REAL NOT NULL,
			confidence REAL NOT NULL,
			input_type TEXT NOT NULL,
			is_public BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL,
			UNIQUE(developer_hash, period, period_start)
		)`,

		`CREATE TABLE IF NOT EXISTS leaderboard_cache (
			id TEXT PRIMARY KEY,
			cache_key TEXT NOT NULL UNIQUE,
			cache_data TEXT NOT NULL, -- JSON data
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL
		)`,

		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_users_ip ON users(ip_address)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_user_id ON request_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_developer_analyses_hash ON developer_analyses(developer_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_developer_analyses_score ON developer_analyses(score DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_developer_analyses_created ON developer_analyses(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_leaderboard_entries_period ON leaderboard_entries(period, period_start)`,
		`CREATE INDEX IF NOT EXISTS idx_leaderboard_entries_rank ON leaderboard_entries(period, period_start, rank)`,
		`CREATE INDEX IF NOT EXISTS idx_leaderboard_cache_key ON leaderboard_cache(cache_key)`,
		`CREATE INDEX IF NOT EXISTS idx_leaderboard_cache_expires ON leaderboard_cache(expires_at)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
