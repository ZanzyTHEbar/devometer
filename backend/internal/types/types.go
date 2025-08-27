package types

import "time"

// RawEvent represents an unprocessed event from adapters
type RawEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Count     float64                `json:"count"`
	Repo      string                 `json:"repo"`
	Language  string                 `json:"language"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// AnalyzeRequest represents the request structure for analyze endpoint
type AnalyzeRequest struct {
	Input string `json:"input" binding:"required"`
}
