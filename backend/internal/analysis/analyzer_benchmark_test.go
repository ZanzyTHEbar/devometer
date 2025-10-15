package analysis

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
)

// BenchmarkAnalyzerGitHub benchmarks the full GitHub analysis pipeline
func BenchmarkAnalyzerGitHub(b *testing.B) {
	// Create analyzer
	analyzer := NewAnalyzer("/tmp/test")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create test events using the correct RawEvent structure
		githubEvents := []types.RawEvent{
			{
				Type:      "github",
				Timestamp: time.Now(),
				Count:     1.0,
				Repo:      "test/repo",
				Metadata: map[string]interface{}{
					"stars":        100,
					"forks":        20,
					"commits":      50,
					"issues":       5,
					"contributors": 10,
					"languages":    map[string]interface{}{"Go": 80000, "JavaScript": 20000},
				},
			},
		}

		result, err := analyzer.AnalyzeEvents(githubEvents, "github")
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}

		// Verify result structure
		if result.Score <= 0 {
			b.Errorf("Invalid score: %d", result.Score)
		}
		if result.Confidence <= 0 {
			b.Errorf("Invalid confidence: %f", result.Confidence)
		}
	}
}

// BenchmarkAnalyzerX benchmarks the X (Twitter) analysis pipeline
func BenchmarkAnalyzerX(b *testing.B) {
	// Create analyzer
	analyzer := NewAnalyzer("/tmp/test")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create test events using the correct RawEvent structure
		xEvents := []types.RawEvent{
			{
				Type:      "x",
				Timestamp: time.Now(),
				Count:     1.0,
				Repo:      "testuser",
				Metadata: map[string]interface{}{
					"followers":    1000,
					"following":    200,
					"tweet_count":  500,
					"listed_count": 50,
					"engagement":   35, // likes + retweets + quotes
					"tweets":       []map[string]interface{}{{"text": "Test tweet", "likes": 10}},
				},
			},
		}

		result, err := analyzer.AnalyzeEvents(xEvents, "x")
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}

		// Verify result structure
		if result.Score <= 0 {
			b.Errorf("Invalid score: %d", result.Score)
		}
		if result.Confidence <= 0 {
			b.Errorf("Invalid confidence: %f", result.Confidence)
		}
	}
}

// BenchmarkAnalyzerCombined benchmarks the combined GitHub + X analysis
func BenchmarkAnalyzerCombined(b *testing.B) {
	// Create analyzer
	analyzer := NewAnalyzer("/tmp/test")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create test events for both platforms
		githubEvents := []types.RawEvent{
			{
				Type:      "github",
				Timestamp: time.Now(),
				Count:     1.0,
				Repo:      "test/repo",
				Metadata: map[string]interface{}{
					"stars":        100,
					"forks":        20,
					"commits":      50,
					"issues":       5,
					"contributors": 10,
					"languages":    map[string]interface{}{"Go": 80000, "JavaScript": 20000},
				},
			},
		}

		xEvents := []types.RawEvent{
			{
				Type:      "x",
				Timestamp: time.Now(),
				Count:     1.0,
				Repo:      "testuser",
				Metadata: map[string]interface{}{
					"followers":    1000,
					"following":    200,
					"tweet_count":  500,
					"listed_count": 50,
					"engagement":   35,
					"tweets":       []map[string]interface{}{{"text": "Test tweet", "likes": 10}},
				},
			},
		}

		result, err := analyzer.AnalyzeEventsWithX(githubEvents, xEvents, "combined")
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}

		// Verify result structure
		if result.Score <= 0 {
			b.Errorf("Invalid score: %d", result.Score)
		}
		if result.Confidence <= 0 {
			b.Errorf("Invalid confidence: %f", result.Confidence)
		}
	}
}

// BenchmarkAnalysisParallel benchmarks parallel analysis processing
func BenchmarkAnalysisParallel(b *testing.B) {
	// Create analyzer
	analyzer := NewAnalyzer("/tmp/test")

	// Create multiple analysis events
	events := []types.RawEvent{
		{
			Type:      "github",
			Timestamp: time.Now(),
			Count:     1.0,
			Repo:      "test1/repo1",
			Metadata: map[string]interface{}{
				"stars": 100, "forks": 20, "commits": 50, "issues": 5, "contributors": 10,
			},
		},
		{
			Type:      "github",
			Timestamp: time.Now(),
			Count:     1.0,
			Repo:      "test2/repo2",
			Metadata: map[string]interface{}{
				"stars": 200, "forks": 40, "commits": 100, "issues": 10, "contributors": 20,
			},
		},
		{
			Type:      "x",
			Timestamp: time.Now(),
			Count:     1.0,
			Repo:      "user1",
			Metadata: map[string]interface{}{
				"followers": 1000, "following": 200, "tweet_count": 500, "engagement": 35,
			},
		},
		{
			Type:      "x",
			Timestamp: time.Now(),
			Count:     1.0,
			Repo:      "user2",
			Metadata: map[string]interface{}{
				"followers": 2000, "following": 400, "tweet_count": 1000, "engagement": 70,
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Analyze different events (simulate real usage pattern)
			testEvents := []types.RawEvent{events[i%len(events)]}

			result, err := analyzer.AnalyzeEvents(testEvents, events[i%len(events)].Type)
			if err != nil {
				b.Fatalf("Analysis failed: %v", err)
			}

			// Verify result structure
			if result.Score <= 0 {
				b.Errorf("Invalid score: %d", result.Score)
			}

			i++
		}
	})
}

// BenchmarkAnalysisMemory benchmarks memory usage during analysis
func BenchmarkAnalysisMemory(b *testing.B) {
	// Create analyzer
	analyzer := NewAnalyzer("/tmp/test")

	// Create test events
	events := []types.RawEvent{
		{
			Type:      "github",
			Timestamp: time.Now(),
			Count:     1.0,
			Repo:      "test/repo",
			Metadata: map[string]interface{}{
				"stars": 100, "forks": 20, "commits": 50, "issues": 5, "contributors": 10,
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := analyzer.AnalyzeEvents(events, "github")
		if err != nil {
			b.Fatalf("Analysis failed: %v", err)
		}

		// Verify result structure
		if result.Score <= 0 {
			b.Errorf("Invalid score: %d", result.Score)
		}

		// Force garbage collection to measure memory pressure
		if i%100 == 0 {
			time.Sleep(time.Millisecond) // Allow GC to run
		}
	}
}

// BenchmarkResultMarshaling benchmarks JSON marshaling performance
func BenchmarkResultMarshaling(b *testing.B) {
	// Create a sample result
	result := ScoreResult{
		Score:      75,
		Confidence: 0.85,
		Posterior:  0.92,
		Breakdown: Breakdown{
			Shipping:      80.0,
			Quality:       70.0,
			Influence:     75.0,
			Complexity:    60.0,
			Collaboration: 85.0,
			Reliability:   90.0,
			Novelty:       65.0,
		},
		Contributors: []Contributor{
			{Name: "Alice", Contribution: 0.3},
			{Name: "Bob", Contribution: 0.25},
			{Name: "Charlie", Contribution: 0.2},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(result)
		if err != nil {
			b.Fatalf("Marshaling failed: %v", err)
		}

		// Verify we have valid JSON
		if len(data) == 0 {
			b.Error("Empty JSON result")
		}
	}
}

// BenchmarkCachePerformance benchmarks cache hit/miss performance
func BenchmarkCachePerformance(b *testing.B) {
	// This would benchmark the cache middleware
	// For now, we'll create a simple cache performance test

	// Create a simple in-memory cache simulation
	cache := make(map[string][]byte)
	hitCount := 0
	missCount := 0

	// Pre-populate some cache entries
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		cache[key] = []byte(fmt.Sprintf("value_%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%150) // Some hits, some misses

		if _, found := cache[key]; found {
			hitCount++
		} else {
			missCount++
		}
	}

	b.Logf("Cache hits: %d, misses: %d", hitCount, missCount)
}

// BenchmarkDatabaseQueries benchmarks database query performance
func BenchmarkDatabaseQueries(b *testing.B) {
	// This would benchmark database operations
	// For now, we'll create a simple query simulation

	queryTemplates := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO request_logs (id, user_id, endpoint) VALUES (?, ?, ?)",
		"SELECT COUNT(*) FROM developer_analyses WHERE developer_hash = ?",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		template := queryTemplates[i%len(queryTemplates)]

		// Simulate query preparation and execution
		// In real implementation, this would be actual database queries
		_ = fmt.Sprintf(template, "test_param")

		// Simulate result processing
		if i%10 == 0 {
			time.Sleep(time.Microsecond) // Simulate I/O delay
		}
	}
}
