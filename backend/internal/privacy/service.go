package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/database"
)

// PrivacyService handles data anonymization and privacy compliance
type PrivacyService struct {
	db *database.DB
}

// NewService creates a new privacy service
func NewService(db *database.DB) *PrivacyService {
	return &PrivacyService{db: db}
}

// AnonymizeData creates anonymized versions of user data
func (ps *PrivacyService) AnonymizeData(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// IsPublicData checks if data can be made public based on privacy rules
func (ps *PrivacyService) IsPublicData(input string, inputType string) bool {
	// Default privacy rules:
	// - GitHub repositories are generally public
	// - GitHub users may prefer privacy
	// - X (Twitter) accounts should be anonymized by default
	// - Combined data requires explicit consent

	switch inputType {
	case "github-repo":
		return true // Repositories are typically public
	case "github-user":
		return false // Users need explicit consent
	case "x-account":
		return false // Social media needs explicit consent
	case "combined":
		return false // Combined data needs explicit consent
	default:
		return false // Default to private
	}
}

// DeleteUserData removes all data associated with a developer hash
func (ps *PrivacyService) DeleteUserData(developerHash string) error {
	slog.Info("Initiating GDPR-compliant data deletion", "developer_hash", developerHash[:8]+"...")

	// Delete from developer_analyses
	analysisQuery := "DELETE FROM developer_analyses WHERE developer_hash = ?"
	analysisResult, err := ps.db.Exec(analysisQuery, developerHash)
	if err != nil {
		return fmt.Errorf("failed to delete developer analyses: %w", err)
	}

	analysisRows, _ := analysisResult.RowsAffected()

	// Delete from leaderboard_entries
	leaderboardQuery := "DELETE FROM leaderboard_entries WHERE developer_hash = ?"
	leaderboardResult, err := ps.db.Exec(leaderboardQuery, developerHash)
	if err != nil {
		return fmt.Errorf("failed to delete leaderboard entries: %w", err)
	}

	leaderboardRows, _ := leaderboardResult.RowsAffected()

	// Clean up cache entries
	cacheQuery := "DELETE FROM leaderboard_cache WHERE cache_key LIKE ?"
	cacheKeyPattern := "%" + developerHash[:8] + "%"
	cacheResult, err := ps.db.Exec(cacheQuery, cacheKeyPattern)
	if err != nil {
		slog.Warn("Failed to clean cache entries", "error", err)
	}

	cacheRows, _ := cacheResult.RowsAffected()

	slog.Info("Data deletion completed",
		"developer_hash", developerHash[:8]+"...",
		"analyses_deleted", analysisRows,
		"leaderboard_entries_deleted", leaderboardRows,
		"cache_entries_deleted", cacheRows,
	)

	return nil
}

// GetDataRetentionInfo provides information about data retention policies
func (ps *PrivacyService) GetDataRetentionInfo() map[string]interface{} {
	return map[string]interface{}{
		"analysis_data_retention_days": 365, // 1 year for analysis data
		"leaderboard_retention_days":   90,  // 90 days for leaderboard rankings
		"cache_retention_minutes":      15,  // 15 minutes for cached data
		"anonymization_method":         "SHA-256",
		"data_deletion_response_time":  "24 hours",
		"privacy_policy_url":           "/privacy-policy",
		"contact_email":                "privacy@cracked-dev-meter.com",
	}
}

// ScheduleDataCleanup schedules automatic cleanup of old data
func (ps *PrivacyService) ScheduleDataCleanup(retentionDays int) error {
	slog.Info("Scheduling data cleanup", "retention_days", retentionDays)

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old developer analyses
	analysisQuery := "DELETE FROM developer_analyses WHERE created_at < ? AND is_public = FALSE"
	analysisResult, err := ps.db.Exec(analysisQuery, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to delete old analyses: %w", err)
	}

	analysisRows, _ := analysisResult.RowsAffected()

	// Note: We keep public leaderboard data longer for historical rankings
	// Only delete non-public data that's older than retention period

	slog.Info("Data cleanup completed", "cutoff_date", cutoffDate, "analyses_deleted", analysisRows)
	return nil
}

// ValidatePrivacyConsent checks if user has given proper consent for data usage
func (ps *PrivacyService) ValidatePrivacyConsent(input string, inputType string, consentGiven bool) bool {
	if consentGiven {
		return true
	}

	// Check if data type allows public display without explicit consent
	return ps.IsPublicData(input, inputType)
}

// GetPrivacySettings returns current privacy settings for a developer
func (ps *PrivacyService) GetPrivacySettings(developerHash string) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_analyses,
			SUM(CASE WHEN is_public = 1 THEN 1 ELSE 0 END) as public_analyses,
			MAX(created_at) as last_analysis_date,
			MIN(created_at) as first_analysis_date
		FROM developer_analyses
		WHERE developer_hash = ?
	`

	var totalAnalyses, publicAnalyses int
	var lastAnalysisDate, firstAnalysisDate *time.Time

	err := ps.db.QueryRow(query, developerHash).Scan(
		&totalAnalyses, &publicAnalyses, &lastAnalysisDate, &firstAnalysisDate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get privacy settings: %w", err)
	}

	return map[string]interface{}{
		"developer_hash":      developerHash[:8] + "...",
		"total_analyses":      totalAnalyses,
		"public_analyses":     publicAnalyses,
		"private_analyses":    totalAnalyses - publicAnalyses,
		"last_analysis_date":  lastAnalysisDate,
		"first_analysis_date": firstAnalysisDate,
		"data_retention_info": ps.GetDataRetentionInfo(),
		"can_delete_data":     true,
	}, nil
}

// UpdatePrivacySettings updates privacy settings for a developer
func (ps *PrivacyService) UpdatePrivacySettings(developerHash string, isPublic bool) error {
	query := `
		UPDATE developer_analyses
		SET is_public = ?, updated_at = ?
		WHERE developer_hash = ?
	`

	now := time.Now()
	result, err := ps.db.Exec(query, isPublic, now, developerHash)
	if err != nil {
		return fmt.Errorf("failed to update privacy settings: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	slog.Info("Privacy settings updated",
		"developer_hash", developerHash[:8]+"...",
		"is_public", isPublic,
		"rows_affected", rowsAffected,
	)

	return nil
}
