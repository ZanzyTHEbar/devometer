package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection with pooling
type DB struct {
	*sql.DB
	pool     *ConnectionPool
	prepared map[string]*sql.Stmt
	mutex    sync.RWMutex
}

// ConnectionPool manages database connection pooling
type ConnectionPool struct {
	db           *sql.DB
	maxOpenConns int
	maxIdleConns int
	maxLifetime  time.Duration
}

// NewConnectionPool creates a new database connection pool
func NewConnectionPool(db *sql.DB, maxOpen, maxIdle int, maxLifetime time.Duration) *ConnectionPool {
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)

	return &ConnectionPool{
		db:           db,
		maxOpenConns: maxOpen,
		maxIdleConns: maxIdle,
		maxLifetime:  maxLifetime,
	}
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	stats := cp.db.Stats()

	return map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"max_open_connections": cp.maxOpenConns,
		"max_idle_connections": cp.maxIdleConns,
		"max_lifetime_seconds": cp.maxLifetime.Seconds(),
		"wait_count":           stats.WaitCount,
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}

// NewDB creates a new database connection with optimized pooling
func NewDB(dataDir string) (*DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "cracked_dev_meter.db")

	// Configure connection string for better performance
	connStr := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pooling for better performance
	pool := NewConnectionPool(db, 25, 5, 5*time.Minute) // 25 max open, 5 idle, 5min lifetime

	database := &DB{
		DB:       db,
		pool:     pool,
		prepared: make(map[string]*sql.Stmt),
	}

	// Run migrations
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize prepared statements
	if err := database.initPreparedStatements(); err != nil {
		return nil, fmt.Errorf("failed to initialize prepared statements: %w", err)
	}

	slog.Info("Database initialized with connection pooling",
		"max_open_conns", pool.maxOpenConns,
		"max_idle_conns", pool.maxIdleConns,
		"max_lifetime", pool.maxLifetime)

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

// initPreparedStatements initializes frequently used prepared statements
func (db *DB) initPreparedStatements() error {
	statements := map[string]string{
		"insert_user": `INSERT INTO users (id, email, ip_address, user_agent, is_paid, stripe_customer_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET
			email = excluded.email,
			ip_address = excluded.ip_address,
			user_agent = excluded.user_agent,
			is_paid = excluded.is_paid,
			stripe_customer_id = excluded.stripe_customer_id,
			updated_at = excluded.updated_at`,

		"insert_request_log": `INSERT INTO request_logs (id, user_id, ip_address, endpoint, method, user_agent, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,

		"insert_payment": `INSERT INTO payments (id, user_id, stripe_payment_id, amount, currency, status, type, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,

		"insert_analysis": `INSERT INTO developer_analyses (
			id, developer_hash, input_type, input_value, score, confidence, posterior,
			breakdown, github_username, x_username, ip_address, user_agent,
			is_public, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(developer_hash) DO UPDATE SET
			score = excluded.score,
			confidence = excluded.confidence,
			posterior = excluded.posterior,
			breakdown = excluded.breakdown,
			updated_at = excluded.updated_at`,

		"insert_leaderboard_entry": `INSERT INTO leaderboard_entries (
			id, developer_hash, period, period_start, period_end, rank,
			score, confidence, input_type, is_public, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,

		"get_user_by_ip": `SELECT id, email, is_paid, stripe_customer_id, created_at, updated_at
			FROM users WHERE ip_address = ? ORDER BY created_at DESC LIMIT 1`,

		"get_request_logs": `SELECT id, user_id, endpoint, method, created_at
			FROM request_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT 10`,

		"get_analyses_by_hash": `SELECT id, score, confidence, posterior, breakdown, input_type, created_at
			FROM developer_analyses WHERE developer_hash = ? ORDER BY created_at DESC`,

		"get_leaderboard": `SELECT id, developer_hash, period, period_start, period_end, rank,
			score, confidence, input_type, is_public, created_at
			FROM leaderboard_entries WHERE period = ? ORDER BY rank ASC LIMIT ?`,

		"get_leaderboard_rank": `SELECT id, developer_hash, period, period_start, period_end, rank,
			score, confidence, input_type, is_public, created_at
			FROM leaderboard_entries WHERE developer_hash = ? AND period = ?`,
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	for name, query := range statements {
		stmt, err := db.Prepare(query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement %s: %w", name, err)
		}
		db.prepared[name] = stmt

		slog.Debug("Prepared statement initialized", "name", name)
	}

	return nil
}

// GetPreparedStatement retrieves a prepared statement
func (db *DB) GetPreparedStatement(name string) (*sql.Stmt, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	stmt, exists := db.prepared[name]
	if !exists {
		return nil, fmt.Errorf("prepared statement %s not found", name)
	}

	return stmt, nil
}

// GetPoolStats returns database connection pool statistics
func (db *DB) GetPoolStats() map[string]interface{} {
	return db.pool.GetStats()
}

// Close closes the database connection and prepared statements
func (db *DB) Close() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Close all prepared statements
	for name, stmt := range db.prepared {
		if err := stmt.Close(); err != nil {
			slog.Warn("Failed to close prepared statement", "name", name, "error", err)
		}
	}

	// Clear the map
	db.prepared = make(map[string]*sql.Stmt)

	// Close the database connection
	return db.DB.Close()
}
