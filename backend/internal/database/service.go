package database

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserService provides business logic for user management
type UserService struct {
	repo      *Repository
	jwtSecret []byte
	freeLimit int
}

// NewUserService creates a new user service
func NewUserService(repo *Repository, jwtSecret string) *UserService {
	return &UserService{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		freeLimit: 5, // 5 free requests per week
	}
}

// ProcessRequest processes an API request and handles rate limiting
func (s *UserService) ProcessRequest(ipAddress, userAgent, endpoint, method string) (*RequestResult, error) {
	// Get or create user
	user, err := s.repo.GetOrCreateUser(ipAddress, userAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create user: %w", err)
	}

	// Check if user can make a request
	canMakeRequest, usage, err := s.repo.CanMakeRequest(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check request limits: %w", err)
	}

	result := &RequestResult{
		User:           user,
		Usage:          usage,
		CanMakeRequest: canMakeRequest,
	}

	// If this is an analyze endpoint request, log it
	if endpoint == "/analyze" || endpoint == "/api/analyze" {
		if canMakeRequest {
			// Log the request
			err = s.repo.LogRequest(user.ID, ipAddress, endpoint, method, userAgent)
			if err != nil {
				return nil, fmt.Errorf("failed to log request: %w", err)
			}
			result.RequestLogged = true
		} else {
			result.RequestLogged = false
		}
	}

	return result, nil
}

// RequestResult represents the result of processing a request
type RequestResult struct {
	User           *User       `json:"user"`
	Usage          *UsageStats `json:"usage"`
	CanMakeRequest bool        `json:"can_make_request"`
	RequestLogged  bool        `json:"request_logged"`
}

// GetRemainingRequests returns the number of remaining requests for the user
func (s *UserService) GetRemainingRequests(userID string) (int, error) {
	usage, err := s.repo.GetWeeklyUsage(userID)
	if err != nil {
		return 0, err
	}

	if usage.IsPaid {
		return -1, nil // Unlimited for paid users
	}

	remaining := s.freeLimit - usage.RequestsThisWeek
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// GenerateSessionToken generates a JWT token for the user session
func (s *UserService) GenerateSessionToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(), // 24 hour expiry
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return tokenString, nil
}

// ValidateSessionToken validates a JWT token and returns the user ID
func (s *UserService) ValidateSessionToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			return "", fmt.Errorf("user_id not found in token")
		}
		return userID, nil
	}

	return "", fmt.Errorf("invalid token")
}

// UpgradeUserToPaid upgrades a user to paid status
func (s *UserService) UpgradeUserToPaid(userID, stripeCustomerID string) error {
	return s.repo.UpdateUserPaymentStatus(userID, true, stripeCustomerID)
}

// CreatePaymentRecord creates a payment record in the database
func (s *UserService) CreatePaymentRecord(userID, stripePaymentID, currency, status, paymentType string, amount int64) (*Payment, error) {
	return s.repo.CreatePayment(userID, stripePaymentID, currency, status, paymentType, amount)
}

// GetUserStats returns comprehensive user statistics
func (s *UserService) GetUserStats(userID string) (*UserStats, error) {
	usage, err := s.repo.GetWeeklyUsage(userID)
	if err != nil {
		return nil, err
	}

	remaining, err := s.GetRemainingRequests(userID)
	if err != nil {
		return nil, err
	}

	return &UserStats{
		UserID:            userID,
		RequestsThisWeek:  usage.RequestsThisWeek,
		RemainingRequests: remaining,
		IsPaid:            usage.IsPaid,
		WeekStart:         usage.WeekStart,
		WeekEnd:           usage.WeekEnd,
	}, nil
}

// UserStats represents comprehensive user statistics
type UserStats struct {
	UserID            string    `json:"user_id"`
	RequestsThisWeek  int       `json:"requests_this_week"`
	RemainingRequests int       `json:"remaining_requests"` // -1 for unlimited
	IsPaid            bool      `json:"is_paid"`
	WeekStart         time.Time `json:"week_start"`
	WeekEnd           time.Time `json:"week_end"`
}
