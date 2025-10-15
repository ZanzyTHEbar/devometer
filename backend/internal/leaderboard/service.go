package leaderboard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/analysis"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/database"
	"github.com/google/uuid"
)

// DeveloperAnalysis represents a stored analysis result
type DeveloperAnalysis struct {
	ID             string    `json:"id"`
	DeveloperHash  string    `json:"developer_hash"`
	InputType      string    `json:"input_type"`
	InputValue     string    `json:"input_value"`
	Score          float64   `json:"score"`
	Confidence     float64   `json:"confidence"`
	Posterior      float64   `json:"posterior"`
	Breakdown      string    `json:"breakdown"` // JSON string
	GitHubUsername *string   `json:"github_username,omitempty"`
	XUsername      *string   `json:"x_username,omitempty"`
	IPAddress      string    `json:"ip_address"`
	UserAgent      string    `json:"user_agent"`
	IsPublic       bool      `json:"is_public"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// LeaderboardEntry represents a leaderboard ranking
type LeaderboardEntry struct {
	ID             string    `json:"id"`
	DeveloperHash  string    `json:"developer_hash"`
	DisplayName    *string   `json:"display_name,omitempty"`
	GitHubUsername *string   `json:"github_username,omitempty"`
	XUsername      *string   `json:"x_username,omitempty"`
	Period         string    `json:"period"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	Rank           int       `json:"rank"`
	Score          float64   `json:"score"`
	Confidence     float64   `json:"confidence"`
	InputType      string    `json:"input_type"`
	IsPublic       bool      `json:"is_public"`
	CreatedAt      time.Time `json:"created_at"`
}

// LeaderboardResponse represents the response for leaderboard queries
type LeaderboardResponse struct {
	Entries     []LeaderboardEntry `json:"entries"`
	Total       int                `json:"total"`
	Period      string             `json:"period"`
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
}

// Service handles leaderboard operations
type Service struct {
	db    *database.DB
	cache *LeaderboardCache
}

// NewService creates a new leaderboard service
func NewService(db *database.DB) *Service {
	return &Service{
		db:    db,
		cache: NewLeaderboardCache(15 * time.Minute), // 15 minute cache TTL
	}
}

// NewServiceWithCache creates a new leaderboard service with custom cache
func NewServiceWithCache(db *database.DB, cache *LeaderboardCache) *Service {
	return &Service{
		db:    db,
		cache: cache,
	}
}

// SaveAnalysis saves a developer analysis result
func (s *Service) SaveAnalysis(result analysis.ScoreResult, input, inputType, ipAddress, userAgent string, githubUsername, xUsername *string, displayName string, isPublic bool) error {
	id := uuid.New().String()
	now := time.Now()

	// Create anonymized hash of the input for privacy
	hash := sha256.Sum256([]byte(input))
	developerHash := hex.EncodeToString(hash[:])

	// Marshal breakdown to JSON
	breakdownJSON, err := json.Marshal(result.Breakdown)
	if err != nil {
		return fmt.Errorf("failed to marshal breakdown: %w", err)
	}

	// Default opt-in status to pending
	optInStatus := "pending"
	var optInAt *time.Time

	query := `
		INSERT INTO developer_analyses (
			id, developer_hash, input_type, input_value, score, confidence, posterior,
			breakdown, github_username, x_username, display_name, ip_address, user_agent,
			is_public, leaderboard_opt_in_status, leaderboard_opt_in_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(developer_hash) DO UPDATE SET
			score = excluded.score,
			confidence = excluded.confidence,
			posterior = excluded.posterior,
			breakdown = excluded.breakdown,
			updated_at = excluded.updated_at
	`

	_, err = s.db.Exec(query,
		id, developerHash, inputType, input, result.Score, result.Confidence, result.Posterior,
		string(breakdownJSON), githubUsername, xUsername, displayName, ipAddress, userAgent,
		isPublic, optInStatus, optInAt, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}

	// Save to analysis_history for weighted scoring
	historyID := uuid.New().String()
	historyQuery := `
		INSERT INTO analysis_history (id, developer_hash, analysis_id, score, confidence, input_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err = s.db.Exec(historyQuery, historyID, developerHash, id, result.Score, result.Confidence, inputType, now)
	if err != nil {
		slog.Error("Failed to save analysis history", "error", err)
		// Don't fail the whole operation if history save fails
	}

	slog.Info("Analysis saved to leaderboard",
		"developer_hash", developerHash[:8]+"...",
		"score", result.Score,
		"input_type", inputType,
	)

	return nil
}

// CalculateWeightedScore calculates weighted average score for a developer
func (s *Service) CalculateWeightedScore(developerHash string) (float64, float64, error) {
	query := `
		SELECT score, confidence, input_type, created_at
		FROM analysis_history
		WHERE developer_hash = ?
		ORDER BY created_at DESC
		LIMIT 10
	`

	rows, err := s.db.Query(query, developerHash)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query analysis history: %w", err)
	}
	defer rows.Close()

	var totalWeightedScore, totalWeight float64
	var analyses []struct {
		score      float64
		confidence float64
		inputType  string
		createdAt  time.Time
	}

	for rows.Next() {
		var a struct {
			score      float64
			confidence float64
			inputType  string
			createdAt  time.Time
		}
		if err := rows.Scan(&a.score, &a.confidence, &a.inputType, &a.createdAt); err != nil {
			return 0, 0, err
		}
		analyses = append(analyses, a)
	}

	if len(analyses) == 0 {
		return 0, 0, fmt.Errorf("no analyses found for developer")
	}

	// Weight calculation:
	// - More recent analyses have higher weight (exponential decay)
	// - Higher confidence analyses have higher weight
	// - Combined analyses get 1.5x weight multiplier
	for i, a := range analyses {
		// Time decay: newer = higher weight (0.5 to 1.0 based on position)
		timeWeight := 1.0 - (float64(i) / float64(len(analyses)) * 0.5)

		// Confidence weight (0.5 to 1.0 based on confidence)
		confidenceWeight := 0.5 + (a.confidence * 0.5)

		// Input type multiplier
		typeMultiplier := 1.0
		if a.inputType == "combined" {
			typeMultiplier = 1.5
		}

		// Combined weight
		weight := timeWeight * confidenceWeight * typeMultiplier
		totalWeightedScore += a.score * weight
		totalWeight += weight
	}

	weightedScore := totalWeightedScore / totalWeight
	avgConfidence := 0.0
	for _, a := range analyses {
		avgConfidence += a.confidence
	}
	avgConfidence /= float64(len(analyses))

	return weightedScore, avgConfidence, nil
}

// UpdateTop10Immediately updates top 10 leaderboard immediately for a developer
func (s *Service) UpdateTop10Immediately(developerHash string, period string) error {
	// Calculate new weighted score
	weightedScore, _, err := s.CalculateWeightedScore(developerHash)
	if err != nil {
		return fmt.Errorf("failed to calculate weighted score: %w", err)
	}

	// Check if this score would place in top 10
	query := `
		SELECT COUNT(*) FROM leaderboard_entries
		WHERE period = ? AND score > ? AND rank <= 10
	`

	var countAbove int
	err = s.db.QueryRow(query, period, weightedScore).Scan(&countAbove)
	if err != nil {
		return err
	}

	// If not in top 10 range, skip immediate update
	if countAbove >= 10 {
		return nil
	}

	// Recalculate top 10 ranks for this period
	return s.updateTop10ForPeriod(period)
}

// updateTop10ForPeriod updates top 10 leaderboard for a specific period
func (s *Service) updateTop10ForPeriod(period string) error {
	now := time.Now()
	var periodStart, periodEnd time.Time

	switch period {
	case "daily":
		periodStart = now.Truncate(24 * time.Hour)
		periodEnd = periodStart.Add(24 * time.Hour).Add(-time.Nanosecond)
	case "weekly":
		days := int(now.Weekday()-time.Monday) % 7
		periodStart = now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
		periodEnd = periodStart.Add(7*24*time.Hour - time.Nanosecond)
	case "monthly":
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "all_time":
		periodStart = time.Date(2020, 1, 1, 0, 0, 0, 0, now.Location())
		periodEnd = now
	default:
		return fmt.Errorf("invalid period: %s", period)
	}

	// Get current top 10 with weighted scores
	query := `
		SELECT da.developer_hash, da.input_type, da.github_username, da.x_username, da.display_name
		FROM developer_analyses da
		WHERE da.is_public = TRUE
		ORDER BY (
			SELECT AVG(ah.score * ah.confidence) 
			FROM analysis_history ah 
			WHERE ah.developer_hash = da.developer_hash
		) DESC
		LIMIT 10
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query top 10: %w", err)
	}
	defer rows.Close()

	// Clear existing top 10 entries for this period
	_, err = s.db.Exec(`DELETE FROM leaderboard_entries WHERE period = ? AND period_start = ? AND rank <= 10`,
		period, periodStart.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("failed to clear top 10 entries: %w", err)
	}

	rank := 1
	for rows.Next() {
		var developerHash, inputType string
		var githubUsername, xUsername, displayName *string

		if err := rows.Scan(&developerHash, &inputType, &githubUsername, &xUsername, &displayName); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Calculate weighted score for this developer
		weightedScore, avgConfidence, err := s.CalculateWeightedScore(developerHash)
		if err != nil {
			slog.Error("Failed to calculate weighted score for top 10", "error", err, "developer_hash", developerHash[:8]+"...")
			continue
		}

		entry := LeaderboardEntry{
			ID:            uuid.New().String(),
			DeveloperHash: developerHash,
			Period:        period,
			PeriodStart:   periodStart,
			PeriodEnd:     periodEnd,
			Rank:          rank,
			Score:         weightedScore,
			Confidence:    avgConfidence,
			InputType:     inputType,
			IsPublic:      true,
			CreatedAt:     now,
		}

		if err := s.saveLeaderboardEntry(entry); err != nil {
			return fmt.Errorf("failed to save top 10 entry: %w", err)
		}

		rank++
	}

	// Invalidate top 10 cache entries
	s.cache.InvalidatePeriod(period)

	slog.Info("Updated top 10 leaderboard", "period", period, "entries", rank-1)
	return nil
}

// UpdateLeaderboards updates all leaderboard rankings for all periods
func (s *Service) UpdateLeaderboards() error {
	periods := []struct {
		name     string
		duration time.Duration
	}{
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
		{"monthly", 30 * 24 * time.Hour},
	}

	now := time.Now()

	for _, period := range periods {
		if err := s.updateLeaderboardForPeriod(period.name, period.duration, now); err != nil {
			slog.Error("Failed to update leaderboard", "period", period.name, "error", err)
			continue
		}
	}

	// Update all-time leaderboard
	if err := s.updateAllTimeLeaderboard(); err != nil {
		slog.Error("Failed to update all-time leaderboard", "error", err)
	}

	// Invalidate cache after leaderboard updates
	s.cache.InvalidateAll()
	slog.Info("Leaderboard cache invalidated after updates")

	return nil
}

// updateLeaderboardForPeriod updates leaderboard for a specific time period
func (s *Service) updateLeaderboardForPeriod(periodName string, duration time.Duration, now time.Time) error {
	var periodStart, periodEnd time.Time

	switch periodName {
	case "daily":
		periodStart = now.Truncate(24 * time.Hour)
		periodEnd = periodStart.Add(24 * time.Hour).Add(-time.Nanosecond)
	case "weekly":
		// Start of week (Monday)
		days := int(now.Weekday()-time.Monday) % 7
		periodStart = now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
		periodEnd = periodStart.Add(7*24*time.Hour - time.Nanosecond)
	case "monthly":
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)
	}

	// Get top scores for this period
	query := `
		SELECT developer_hash, MAX(score) as max_score, AVG(confidence) as avg_confidence, input_type
		FROM developer_analyses
		WHERE created_at >= ? AND created_at <= ? AND is_public = TRUE
		GROUP BY developer_hash, input_type
		ORDER BY max_score DESC, avg_confidence DESC
		LIMIT 100
	`

	rows, err := s.db.Query(query, periodStart, periodEnd)
	if err != nil {
		return fmt.Errorf("failed to query top scores: %w", err)
	}
	defer rows.Close()

	// Clear existing entries for this period
	_, err = s.db.Exec("DELETE FROM leaderboard_entries WHERE period = ? AND period_start = ?",
		periodName, periodStart.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("failed to clear existing entries: %w", err)
	}

	rank := 1
	for rows.Next() {
		var developerHash string
		var maxScore, avgConfidence float64
		var inputType string

		if err := rows.Scan(&developerHash, &maxScore, &avgConfidence, &inputType); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		entry := LeaderboardEntry{
			ID:            uuid.New().String(),
			DeveloperHash: developerHash,
			Period:        periodName,
			PeriodStart:   periodStart,
			PeriodEnd:     periodEnd,
			Rank:          rank,
			Score:         maxScore,
			Confidence:    avgConfidence,
			InputType:     inputType,
			IsPublic:      true,
			CreatedAt:     now,
		}

		if err := s.saveLeaderboardEntry(entry); err != nil {
			return fmt.Errorf("failed to save leaderboard entry: %w", err)
		}

		rank++
	}

	slog.Info("Updated leaderboard", "period", periodName, "entries", rank-1)
	return nil
}

// updateAllTimeLeaderboard updates the all-time leaderboard
func (s *Service) updateAllTimeLeaderboard() error {
	now := time.Now()
	periodStart := time.Date(2020, 1, 1, 0, 0, 0, 0, now.Location()) // Arbitrary start date
	periodEnd := now

	query := `
		SELECT developer_hash, MAX(score) as max_score, AVG(confidence) as avg_confidence, input_type
		FROM developer_analyses
		WHERE is_public = TRUE
		GROUP BY developer_hash, input_type
		ORDER BY max_score DESC, avg_confidence DESC
		LIMIT 100
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query all-time scores: %w", err)
	}
	defer rows.Close()

	// Clear existing all-time entries
	_, err = s.db.Exec("DELETE FROM leaderboard_entries WHERE period = ?", "all_time")
	if err != nil {
		return fmt.Errorf("failed to clear existing all-time entries: %w", err)
	}

	rank := 1
	for rows.Next() {
		var developerHash string
		var maxScore, avgConfidence float64
		var inputType string

		if err := rows.Scan(&developerHash, &maxScore, &avgConfidence, &inputType); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		entry := LeaderboardEntry{
			ID:            uuid.New().String(),
			DeveloperHash: developerHash,
			Period:        "all_time",
			PeriodStart:   periodStart,
			PeriodEnd:     periodEnd,
			Rank:          rank,
			Score:         maxScore,
			Confidence:    avgConfidence,
			InputType:     inputType,
			IsPublic:      true,
			CreatedAt:     now,
		}

		if err := s.saveLeaderboardEntry(entry); err != nil {
			return fmt.Errorf("failed to save all-time leaderboard entry: %w", err)
		}

		rank++
	}

	slog.Info("Updated all-time leaderboard", "entries", rank-1)
	return nil
}

// saveLeaderboardEntry saves a leaderboard entry to the database
func (s *Service) saveLeaderboardEntry(entry LeaderboardEntry) error {
	query := `
		INSERT INTO leaderboard_entries (
			id, developer_hash, period, period_start, period_end, rank,
			score, confidence, input_type, is_public, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		entry.ID, entry.DeveloperHash, entry.Period,
		entry.PeriodStart.Format("2006-01-02"),
		entry.PeriodEnd.Format("2006-01-02"),
		entry.Rank, entry.Score, entry.Confidence, entry.InputType,
		entry.IsPublic, entry.CreatedAt,
	)

	return err
}

// GetLeaderboard retrieves leaderboard entries for a specific period
func (s *Service) GetLeaderboard(period string, limit int) (*LeaderboardResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Try cache first
	if cachedResponse, found := s.cache.GetLeaderboard(period, limit); found {
		return cachedResponse, nil
	}

	var query string
	var args []interface{}
	now := time.Now()

	switch period {
	case "daily":
		periodStart := now.Truncate(24 * time.Hour)
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end,
				le.rank, le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.period = ? AND le.period_start = ?
			ORDER BY le.rank ASC
			LIMIT ?
		`
		args = []interface{}{period, periodStart.Format("2006-01-02"), limit}

	case "weekly":
		days := int(now.Weekday()-time.Monday) % 7
		periodStart := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end,
				le.rank, le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.period = ? AND le.period_start = ?
			ORDER BY le.rank ASC
			LIMIT ?
		`
		args = []interface{}{period, periodStart.Format("2006-01-02"), limit}

	case "monthly":
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end,
				le.rank, le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.period = ? AND le.period_start = ?
			ORDER BY le.rank ASC
			LIMIT ?
		`
		args = []interface{}{period, periodStart.Format("2006-01-02"), limit}

	case "all_time":
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end,
				le.rank, le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.period = ?
			ORDER BY le.rank ASC
			LIMIT ?
		`
		args = []interface{}{period, limit}

	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		var periodStartStr, periodEndStr string

		err := rows.Scan(
			&entry.ID, &entry.DeveloperHash, &entry.Period,
			&periodStartStr, &periodEndStr, &entry.Rank,
			&entry.Score, &entry.Confidence, &entry.InputType,
			&entry.IsPublic, &entry.CreatedAt,
			&entry.DisplayName, &entry.GitHubUsername, &entry.XUsername,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard entry: %w", err)
		}

		if entry.PeriodStart, err = time.Parse("2006-01-02", periodStartStr); err != nil {
			return nil, fmt.Errorf("failed to parse period start: %w", err)
		}
		if entry.PeriodEnd, err = time.Parse("2006-01-02", periodEndStr); err != nil {
			return nil, fmt.Errorf("failed to parse period end: %w", err)
		}

		entries = append(entries, entry)
	}

	response := &LeaderboardResponse{
		Entries: entries,
		Total:   len(entries),
		Period:  period,
	}

	// Set period dates based on the first entry (if any)
	if len(entries) > 0 {
		response.PeriodStart = entries[0].PeriodStart
		response.PeriodEnd = entries[0].PeriodEnd
	}

	// Cache the response for future requests
	s.cache.SetLeaderboard(period, limit, response)

	return response, nil
}

// GetDeveloperRank gets a specific developer's rank in a period
func (s *Service) GetDeveloperRank(developerHash, period string) (*LeaderboardEntry, error) {
	// Try cache first
	if cachedEntry, found := s.cache.GetDeveloperRank(developerHash, period); found {
		return cachedEntry, nil
	}

	var query string
	var args []interface{}
	now := time.Now()

	switch period {
	case "daily":
		periodStart := now.Truncate(24 * time.Hour)
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end, le.rank,
				le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.developer_hash = ? AND le.period = ? AND le.period_start = ?
		`
		args = []interface{}{developerHash, period, periodStart.Format("2006-01-02")}

	case "weekly":
		days := int(now.Weekday()-time.Monday) % 7
		periodStart := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end, le.rank,
				le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.developer_hash = ? AND le.period = ? AND le.period_start = ?
		`
		args = []interface{}{developerHash, period, periodStart.Format("2006-01-02")}

	case "monthly":
		periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end, le.rank,
				le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.developer_hash = ? AND le.period = ? AND le.period_start = ?
		`
		args = []interface{}{developerHash, period, periodStart.Format("2006-01-02")}

	case "all_time":
		query = `
			SELECT 
				le.id, le.developer_hash, le.period, le.period_start, le.period_end, le.rank,
				le.score, le.confidence, le.input_type, le.is_public, le.created_at,
				da.display_name, da.github_username, da.x_username
			FROM leaderboard_entries le
			LEFT JOIN developer_analyses da ON le.developer_hash = da.developer_hash
			WHERE le.developer_hash = ? AND le.period = ?
		`
		args = []interface{}{developerHash, period}

	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	var entry LeaderboardEntry
	var periodStartStr, periodEndStr string

	err := s.db.QueryRow(query, args...).Scan(
		&entry.ID, &entry.DeveloperHash, &entry.Period,
		&periodStartStr, &periodEndStr, &entry.Rank,
		&entry.Score, &entry.Confidence, &entry.InputType,
		&entry.IsPublic, &entry.CreatedAt,
		&entry.DisplayName, &entry.GitHubUsername, &entry.XUsername,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get developer rank: %w", err)
	}

	if entry.PeriodStart, err = time.Parse("2006-01-02", periodStartStr); err != nil {
		return nil, fmt.Errorf("failed to parse period start: %w", err)
	}
	if entry.PeriodEnd, err = time.Parse("2006-01-02", periodEndStr); err != nil {
		return nil, fmt.Errorf("failed to parse period end: %w", err)
	}

	// Cache the entry for future requests
	s.cache.SetDeveloperRank(developerHash, period, &entry)

	return &entry, nil
}

// GetCacheStats returns leaderboard cache statistics
func (s *Service) GetCacheStats() map[string]interface{} {
	return s.cache.GetStats()
}

// WarmCache warms the leaderboard cache with popular data
func (s *Service) WarmCache() {
	s.cache.WarmCache(s)
}

// StartAutoRefresh starts automatic cache refresh
func (s *Service) StartAutoRefresh(interval time.Duration) {
	s.cache.AutoRefresh(s, interval)
}
