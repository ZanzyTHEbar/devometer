package database

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email,omitempty" db:"email"`
	IPAddress string    `json:"-" db:"ip_address"`
	UserAgent string    `json:"-" db:"user_agent"`
	IsPaid    bool      `json:"is_paid" db:"is_paid"`
	StripeID  string    `json:"-" db:"stripe_customer_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// RequestLog tracks API requests for rate limiting
type RequestLog struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	IPAddress string    `json:"-" db:"ip_address"`
	Endpoint  string    `json:"endpoint" db:"endpoint"`
	Method    string    `json:"method" db:"method"`
	UserAgent string    `json:"-" db:"user_agent"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Payment represents a donation/payment
type Payment struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	StripePaymentID string    `json:"stripe_payment_id" db:"stripe_payment_id"`
	Amount          int64     `json:"amount" db:"amount"` // Amount in cents
	Currency        string    `json:"currency" db:"currency"`
	Status          string    `json:"status" db:"status"`
	Type            string    `json:"type" db:"type"` // donation, subscription
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// UsageStats represents weekly usage statistics
type UsageStats struct {
	UserID           string    `json:"user_id"`
	RequestsThisWeek int       `json:"requests_this_week"`
	WeekStart        time.Time `json:"week_start"`
	WeekEnd          time.Time `json:"week_end"`
	IsPaid           bool      `json:"is_paid"`
}

// NewUser creates a new user with generated ID
func NewUser(ipAddress, userAgent string) *User {
	now := time.Now()
	return &User{
		ID:        uuid.New().String(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
		IsPaid:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewRequestLog creates a new request log entry
func NewRequestLog(userID, ipAddress, endpoint, method, userAgent string) *RequestLog {
	return &RequestLog{
		ID:        uuid.New().String(),
		UserID:    userID,
		IPAddress: ipAddress,
		Endpoint:  endpoint,
		Method:    method,
		UserAgent: userAgent,
		CreatedAt: time.Now(),
	}
}
