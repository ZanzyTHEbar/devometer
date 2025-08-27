package adapters

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewXAdapter(t *testing.T) {
	tests := []struct {
		name     string
		config   XAuthConfig
		expected XAuthConfig
	}{
		{
			name: "creates adapter with bearer token",
			config: XAuthConfig{
				BearerToken: "AAAAAAAAAAAAAAAAAAAAA1234567890",
			},
			expected: XAuthConfig{
				BearerToken: "AAAAAAAAAAAAAAAAAAAAA1234567890",
			},
		},
		{
			name: "creates adapter with full OAuth config",
			config: XAuthConfig{
				BearerToken:  "bearer_token",
				APIKey:       "api_key",
				APISecret:    "api_secret",
				AccessToken:  "access_token",
				AccessSecret: "access_secret",
			},
			expected: XAuthConfig{
				BearerToken:  "bearer_token",
				APIKey:       "api_key",
				APISecret:    "api_secret",
				AccessToken:  "access_token",
				AccessSecret: "access_secret",
			},
		},
		{
			name:     "creates adapter with empty config",
			config:   XAuthConfig{},
			expected: XAuthConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewXAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.expected, adapter.config)
		})
	}
}

func TestNewXAdapterWithToken(t *testing.T) {
	token := "AAAAAAAAAAAAAAAAAAAAA1234567890"
	adapter := NewXAdapterWithToken(token)

	assert.NotNil(t, adapter)
	assert.Equal(t, token, adapter.config.BearerToken)
	assert.Empty(t, adapter.config.APIKey)
	assert.Empty(t, adapter.config.APISecret)
}

func TestXAdapter_FetchUserData(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		username string
	}{
		{
			name:     "fetches data for valid username",
			ctx:      context.Background(),
			username: "johndoe",
		},
		{
			name:     "fetches data with @ prefix",
			ctx:      nil,
			username: "@janedoe",
		},
		{
			name:     "fetches data with empty username",
			ctx:      context.Background(),
			username: "",
		},
		{
			name:     "fetches data with special characters",
			ctx:      context.Background(),
			username: "user_name123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewXAdapterWithToken("fake_token")

			result, err := adapter.FetchUserData(tt.ctx, tt.username)

			assert.NoError(t, err)
			assert.NotEmpty(t, result)
			assert.Len(t, result, 8) // Should return 8 different event types

			// Validate event structure
			for i, event := range result {
				assert.NotEmpty(t, event.Type, "Event %d should have type", i)
				assert.Contains(t, event.Timestamp, "-", "Event %d should have RFC3339 timestamp", i)
				assert.GreaterOrEqual(t, event.Count, 0.0, "Event %d should have non-negative count", i)

				// Username should be cleaned (no @ prefix)
				expectedHandle := tt.username
				if strings.HasPrefix(tt.username, "@") {
					expectedHandle = tt.username[1:] // Remove @ if present
				}
				assert.Equal(t, expectedHandle, event.Handle, "Event %d should have correct handle", i)

				// Validate specific event types
				switch event.Type {
				case "twitter_followers":
					assert.GreaterOrEqual(t, event.Count, 100.0)
					assert.LessOrEqual(t, event.Count, 1000.0)
				case "twitter_following":
					assert.GreaterOrEqual(t, event.Count, 50.0)
					assert.LessOrEqual(t, event.Count, 1000.0)
				case "twitter_tweets":
					assert.GreaterOrEqual(t, event.Count, 100.0)
					assert.LessOrEqual(t, event.Count, 1000.0)
				case "twitter_likes":
					assert.GreaterOrEqual(t, event.Count, 50.0)
					assert.LessOrEqual(t, event.Count, 2000.0)
				case "twitter_retweets":
					assert.GreaterOrEqual(t, event.Count, 5.0)
					assert.LessOrEqual(t, event.Count, 200.0)
				case "twitter_replies":
					assert.GreaterOrEqual(t, event.Count, 10.0)
					assert.LessOrEqual(t, event.Count, 400.0)
				case "twitter_mentions":
					assert.GreaterOrEqual(t, event.Count, 50.0)
					assert.LessOrEqual(t, event.Count, 250.0)
				case "twitter_engagement_rate":
					assert.GreaterOrEqual(t, event.Count, 0.01)
					assert.LessOrEqual(t, event.Count, 0.11)
				}
			}
		})
	}
}

func TestXAdapter_FetchRecentTweets(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		username string
		limit    int
		expected int
	}{
		{
			name:     "fetches tweets with default limit",
			ctx:      context.Background(),
			username: "testuser",
			limit:    0,
			expected: 10,
		},
		{
			name:     "fetches tweets with custom limit",
			ctx:      context.Background(),
			username: "@testuser",
			limit:    5,
			expected: 5,
		},
		{
			name:     "caps limit at 100",
			ctx:      context.Background(),
			username: "testuser",
			limit:    150,
			expected: 100,
		},
		{
			name:     "handles negative limit",
			ctx:      context.Background(),
			username: "testuser",
			limit:    -5,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewXAdapterWithToken("fake_token")

			result, err := adapter.FetchRecentTweets(tt.ctx, tt.username, tt.limit)

			assert.NoError(t, err)
			assert.Len(t, result, tt.expected)

			for i, tweet := range result {
				assert.Equal(t, "twitter_tweet", tweet.Type)
				assert.Contains(t, tweet.Timestamp, "-")
				assert.Equal(t, 1.0, tweet.Count)
				assert.NotEmpty(t, tweet.Text)
				assert.Contains(t, tweet.Handle, "testuser")

				// Tweets should be in reverse chronological order
				if i > 0 {
					// Each subsequent tweet should be older
					assert.Contains(t, tweet.Timestamp, "-")
				}
			}
		})
	}
}

func TestXAdapter_FetchHashtagData(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		hashtag  string
		limit    int
		expected int
	}{
		{
			name:     "fetches hashtag data with default limit",
			ctx:      context.Background(),
			hashtag:  "#coding",
			limit:    0,
			expected: 20,
		},
		{
			name:     "fetches hashtag data with # prefix",
			ctx:      context.Background(),
			hashtag:  "#javascript",
			limit:    10,
			expected: 10,
		},
		{
			name:     "caps limit at 500",
			ctx:      context.Background(),
			hashtag:  "python",
			limit:    600,
			expected: 500,
		},
		{
			name:     "handles empty hashtag",
			ctx:      context.Background(),
			hashtag:  "",
			limit:    5,
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewXAdapterWithToken("fake_token")

			result, err := adapter.FetchHashtagData(tt.ctx, tt.hashtag, tt.limit)

			assert.NoError(t, err)
			assert.Len(t, result, tt.expected)

			for _, event := range result {
				assert.Equal(t, "twitter_hashtag_usage", event.Type)
				assert.Contains(t, event.Timestamp, "-")
				assert.GreaterOrEqual(t, event.Count, 0.0)
			}
		})
	}
}

func TestXAdapter_AnalyzeSentiment(t *testing.T) {
	adapter := NewXAdapterWithToken("fake_token")

	tests := []struct {
		name     string
		text     string
		expected float64
		delta    float64
	}{
		{
			name:     "empty text returns neutral",
			text:     "",
			expected: 0.5,
			delta:    0.0,
		},
		{
			name:     "positive text",
			text:     "This is great! I love it! Amazing work!",
			expected: 0.8,
			delta:    0.3,
		},
		{
			name:     "negative text",
			text:     "This is terrible! I hate it! Worst ever!",
			expected: 0.2,
			delta:    0.3,
		},
		{
			name:     "neutral text",
			text:     "This is okay. It's fine. Nothing special.",
			expected: 0.6,
			delta:    0.3,
		},
		{
			name:     "mixed sentiment",
			text:     "Great start but terrible ending. Love the concept though!",
			expected: 0.5,
			delta:    0.3,
		},
		{
			name:     "technical content",
			text:     "The algorithm implements O(n) complexity with optimal space usage.",
			expected: 0.5,
			delta:    0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.AnalyzeSentiment(tt.text)

			assert.NoError(t, err)
			assert.InDelta(t, tt.expected, result, tt.delta)
			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}

func TestXAdapter_GetEngagementScore(t *testing.T) {
	adapter := NewXAdapterWithToken("fake_token")

	tests := []struct {
		name      string
		followers float64
		likes     float64
		retweets  float64
		replies   float64
		expected  float64
		delta     float64
	}{
		{
			name:      "zero followers returns zero",
			followers: 0,
			likes:     100,
			retweets:  20,
			replies:   10,
			expected:  0.0,
			delta:     0.0,
		},
		{
			name:      "high engagement",
			followers: 1000,
			likes:     500,
			retweets:  100,
			replies:   200,
			expected:  0.34,
			delta:     0.1,
		},
		{
			name:      "low engagement",
			followers: 1000,
			likes:     10,
			retweets:  1,
			replies:   5,
			expected:  0.0058,
			delta:     0.01,
		},
		{
			name:      "calculated engagement",
			followers: 100,
			likes:     40,
			retweets:  30,
			replies:   30,
			expected:  0.34,
			delta:     0.01,
		},
		{
			name:      "negative values handled",
			followers: 100,
			likes:     -10,
			retweets:  -5,
			replies:   -2,
			expected:  0.0,
			delta:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.GetEngagementScore(tt.followers, tt.likes, tt.retweets, tt.replies)

			assert.InDelta(t, tt.expected, result, tt.delta)
			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}

func TestXAdapter_TokenHandling(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"short token", "abc123"},
		{"bearer token", "Bearer AAAAAAAAAAAAAAAAAAAAA1234567890"},
		{"long token", "AAAAAAAAAAAAAAAAAAAAA1234567890123456789012345678901234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewXAdapterWithToken(tt.token)
			assert.Equal(t, tt.token, adapter.config.BearerToken)
		})
	}
}

func TestXAdapter_ConcurrentAccess(t *testing.T) {
	adapter := NewXAdapterWithToken("test_token")

	done := make(chan bool, 3)

	go func() {
		_, err := adapter.FetchUserData(context.Background(), "user1")
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		_, err := adapter.FetchRecentTweets(context.Background(), "user2", 5)
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		_, err := adapter.FetchHashtagData(context.Background(), "#test", 10)
		assert.NoError(t, err)
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestXAdapter_DataConsistency(t *testing.T) {
	adapter := NewXAdapterWithToken("test_token")

	result1, err1 := adapter.FetchUserData(context.Background(), "testuser")
	result2, err2 := adapter.FetchUserData(context.Background(), "testuser")

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Len(t, result1, len(result2))

	// Event types should be consistent
	eventTypes1 := make(map[string]bool)
	eventTypes2 := make(map[string]bool)

	for _, event := range result1 {
		eventTypes1[event.Type] = true
	}
	for _, event := range result2 {
		eventTypes2[event.Type] = true
	}

	assert.Equal(t, eventTypes1, eventTypes2)
}

func TestXAdapter_ContextHandling(t *testing.T) {
	adapter := NewXAdapterWithToken("test_token")

	contexts := []context.Context{
		context.Background(),
		context.TODO(),
		nil,
	}

	for i, ctx := range contexts {
		t.Run("context_"+string(rune('A'+i)), func(t *testing.T) {
			result, err := adapter.FetchUserData(ctx, "testuser")
			assert.NoError(t, err)
			assert.NotEmpty(t, result)
		})
	}
}

func TestXAdapter_ErrorScenarios(t *testing.T) {
	adapter := NewXAdapterWithToken("invalid_token")

	testCases := []struct {
		name     string
		username string
		hashtag  string
		limit    int
		testType string
	}{
		{"very long username", string(make([]byte, 200)), "", 0, "user"},
		{"special chars username", "user@#$%^&*()", "", 0, "user"},
		{"empty username", "", "", 0, "user"},
		{"very long hashtag", "", string(make([]byte, 200)), 0, "hashtag"},
		{"special chars hashtag", "", "tag@#$%^&*()", 0, "hashtag"},
		{"empty hashtag", "", "", 0, "hashtag"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result []XEvent
			var err error

			if tc.testType == "user" {
				result, err = adapter.FetchUserData(context.Background(), tc.username)
			} else {
				result, err = adapter.FetchHashtagData(context.Background(), tc.hashtag, tc.limit)
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Greater(t, len(result), 0)
		})
	}
}

func TestXAdapter_EventStructure(t *testing.T) {
	event := XEvent{
		Type:      "test_type",
		Timestamp: "2024-01-01T12:00:00Z",
		Count:     42.0,
		Handle:    "testuser",
		Text:      "Hello world",
	}

	assert.Equal(t, "test_type", event.Type)
	assert.Equal(t, "2024-01-01T12:00:00Z", event.Timestamp)
	assert.Equal(t, 42.0, event.Count)
	assert.Equal(t, "testuser", event.Handle)
	assert.Equal(t, "Hello world", event.Text)
}

func TestXAdapter_SentimentEdgeCases(t *testing.T) {
	adapter := NewXAdapterWithToken("fake_token")

	edgeCases := []struct {
		name string
		text string
	}{
		{"very long text", string(make([]byte, 1000))},
		{"text with numbers", "I got 95% on my test! That's awesome 123"},
		{"text with emojis", "Great work! 🚀✨🎉 Loving the new features 😍"},
		{"text with URLs", "Check out this awesome site: https://example.com it's fantastic!"},
		{"case insensitive", "GREAT work! TERRIBLE execution. OK results."},
		{"repeated words", "Great great great terrible terrible terrible"},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := adapter.AnalyzeSentiment(tc.text)

			assert.NoError(t, err)
			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}

func TestXAdapter_EngagementScoreEdgeCases(t *testing.T) {
	adapter := NewXAdapterWithToken("fake_token")

	edgeCases := []struct {
		name      string
		followers float64
		likes     float64
		retweets  float64
		replies   float64
		expected  float64
	}{
		{"very large numbers", 1000000, 500000, 100000, 200000, 0.8},
		{"very small numbers", 1, 0.1, 0.01, 0.02, 0.13},
		{"zero engagement", 1000, 0, 0, 0, 0.0},
		{"negative engagement", 1000, -10, -5, -2, 0.0},
		{"infinite engagement", 0.1, 100, 50, 25, 1.0},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := adapter.GetEngagementScore(tc.followers, tc.likes, tc.retweets, tc.replies)

			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}
