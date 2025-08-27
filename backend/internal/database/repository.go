package database

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository handles database operations
type Repository struct {
	db *DB
}

// NewRepository creates a new repository
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// GetOrCreateUser gets an existing user or creates a new one based on IP address
func (r *Repository) GetOrCreateUser(ipAddress, userAgent string) (*User, error) {
	// Try to find existing user by IP
	var user User
	err := r.db.QueryRow(`
		SELECT id, email, ip_address, user_agent, is_paid, stripe_customer_id, created_at, updated_at
		FROM users
		WHERE ip_address = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, ipAddress).Scan(
		&user.ID, &user.Email, &user.IPAddress, &user.UserAgent,
		&user.IsPaid, &user.StripeID, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		// User exists, update last seen
		_, err = r.db.Exec(`
			UPDATE users SET updated_at = ?, user_agent = ? WHERE id = ?
		`, time.Now(), userAgent, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// User doesn't exist, create new one
	user = *NewUser(ipAddress, userAgent)
	_, err = r.db.Exec(`
		INSERT INTO users (id, email, ip_address, user_agent, is_paid, stripe_customer_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.IPAddress, user.UserAgent, user.IsPaid, user.StripeID, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// LogRequest logs an API request
func (r *Repository) LogRequest(userID, ipAddress, endpoint, method, userAgent string) error {
	reqLog := NewRequestLog(userID, ipAddress, endpoint, method, userAgent)
	_, err := r.db.Exec(`
		INSERT INTO request_logs (id, user_id, ip_address, endpoint, method, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, reqLog.ID, reqLog.UserID, reqLog.IPAddress, reqLog.Endpoint, reqLog.Method, reqLog.UserAgent, reqLog.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to log request: %w", err)
	}

	return nil
}

// GetWeeklyUsage gets usage statistics for a user for the current week
func (r *Repository) GetWeeklyUsage(userID string) (*UsageStats, error) {
	now := time.Now()

	// Get the start of the current week (Monday)
	weekStart := now.AddDate(0, 0, -int(now.Weekday()-time.Monday))
	if now.Weekday() == time.Sunday {
		weekStart = weekStart.AddDate(0, 0, -7)
	}
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

	weekEnd := weekStart.AddDate(0, 0, 7)

	var requestCount int
	var isPaid bool

	// Get user payment status
	err := r.db.QueryRow(`SELECT is_paid FROM users WHERE id = ?`, userID).Scan(&isPaid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user payment status: %w", err)
	}

	// Count requests for this week
	err = r.db.QueryRow(`
		SELECT COUNT(*) FROM request_logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
	`, userID, weekStart, weekEnd).Scan(&requestCount)

	if err != nil {
		return nil, fmt.Errorf("failed to count requests: %w", err)
	}

	return &UsageStats{
		UserID:           userID,
		RequestsThisWeek: requestCount,
		WeekStart:        weekStart,
		WeekEnd:          weekEnd,
		IsPaid:           isPaid,
	}, nil
}

// CanMakeRequest checks if a user can make another request based on their usage
func (r *Repository) CanMakeRequest(userID string) (bool, *UsageStats, error) {
	usage, err := r.GetWeeklyUsage(userID)
	if err != nil {
		return false, nil, err
	}

	// Paid users have unlimited access
	if usage.IsPaid {
		return true, usage, nil
	}

	// Free users are limited to 5 requests per week
	const freeLimit = 5
	return usage.RequestsThisWeek < freeLimit, usage, nil
}

// UpdateUserPaymentStatus updates a user's payment status
func (r *Repository) UpdateUserPaymentStatus(userID string, isPaid bool, stripeCustomerID string) error {
	_, err := r.db.Exec(`
		UPDATE users SET is_paid = ?, stripe_customer_id = ?, updated_at = ?
		WHERE id = ?
	`, isPaid, stripeCustomerID, time.Now(), userID)

	if err != nil {
		return fmt.Errorf("failed to update user payment status: %w", err)
	}

	return nil
}

// CreatePayment creates a payment record
func (r *Repository) CreatePayment(userID, stripePaymentID, currency, status, paymentType string, amount int64) (*Payment, error) {
	payment := &Payment{
		ID:              NewRequestLog("", "", "", "", "").ID, // Reuse ID generation
		UserID:          userID,
		StripePaymentID: stripePaymentID,
		Amount:          amount,
		Currency:        currency,
		Status:          status,
		Type:            paymentType,
		CreatedAt:       time.Now(),
	}

	_, err := r.db.Exec(`
		INSERT INTO payments (id, user_id, stripe_payment_id, amount, currency, status, type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, payment.ID, payment.UserID, payment.StripePaymentID, payment.Amount,
		payment.Currency, payment.Status, payment.Type, payment.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	return payment, nil
}

// GetUserByStripeCustomerID gets a user by their Stripe customer ID
func (r *Repository) GetUserByStripeCustomerID(stripeCustomerID string) (*User, error) {
	var user User
	err := r.db.QueryRow(`
		SELECT id, email, ip_address, user_agent, is_paid, stripe_customer_id, created_at, updated_at
		FROM users
		WHERE stripe_customer_id = ?
	`, stripeCustomerID).Scan(
		&user.ID, &user.Email, &user.IPAddress, &user.UserAgent,
		&user.IsPaid, &user.StripeID, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get user by stripe customer ID: %w", err)
	}

	return &user, nil
}
