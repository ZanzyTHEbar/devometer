<!-- 348a10bc-2788-4254-a9ed-ac6c6003ef88 fa60347f-0c61-4885-97f1-af57e6b42464 -->
# Leaderboard Feature Implementation Plan

Based on requirements:

- **Display**: Show GitHub username/X handle if opted-in (1.b), fallback to anonymized hash
- **Ranking**: Weighted average of all analyses (2.d)
- **Updates**: Hybrid approach - immediate for top 10, batch for rest (3.c)
- **UI**: Both compact "Top 10" widget and full leaderboard page (4.c)
- **Opt-in**: Post-analysis modal asking "Add to leaderboard?" (5.c)

## Backend Changes

### 1. Database Schema Updates

**File**: `backend/internal/database/db.go`

Add new columns and tables to support the feature:

```go
// In migrate() function, update developer_analyses table:
`CREATE TABLE IF NOT EXISTS developer_analyses (
    id TEXT PRIMARY KEY,
    developer_hash TEXT NOT NULL UNIQUE,
    input_type TEXT NOT NULL,
    input_value TEXT NOT NULL,
    score REAL NOT NULL,
    confidence REAL NOT NULL,
    posterior REAL NOT NULL,
    breakdown TEXT,
    github_username TEXT,
    x_username TEXT,
    display_name TEXT, -- NEW: User-provided display name
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    is_public BOOLEAN DEFAULT FALSE,
    leaderboard_opt_in_status TEXT DEFAULT 'pending', -- NEW: 'pending', 'accepted', 'declined'
    leaderboard_opt_in_at DATETIME, -- NEW: When user opted in/out
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
)`

// Add new table for analysis history (multiple analyses per developer)
`CREATE TABLE IF NOT EXISTS analysis_history (
    id TEXT PRIMARY KEY,
    developer_hash TEXT NOT NULL,
    analysis_id TEXT NOT NULL,
    score REAL NOT NULL,
    confidence REAL NOT NULL,
    input_type TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (developer_hash) REFERENCES developer_analyses(developer_hash),
    FOREIGN KEY (analysis_id) REFERENCES developer_analyses(id)
)`

// Add index for weighted average queries
`CREATE INDEX IF NOT EXISTS idx_analysis_history_hash ON analysis_history(developer_hash)`
```

### 2. Leaderboard Service - Weighted Scoring

**File**: `backend/internal/leaderboard/service.go`

Add weighted average calculation:

```go
// Add new method to calculate weighted average score
func (s *Service) CalculateWeightedScore(developerHash string) (float64, float64, error) {
    query := `
        SELECT score, confidence, input_type, created_at
        FROM analysis_history
        WHERE developer_hash = ?
        ORDER BY created_at DESC
        LIMIT 10 -- Last 10 analyses
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
    now := time.Now()
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

// Update SaveAnalysis to store in analysis_history
func (s *Service) SaveAnalysis(result analysis.ScoreResult, input, inputType, ipAddress, userAgent string, 
    githubUsername, xUsername *string, displayName string, isPublic bool) error {
    
    // ... existing code to save to developer_analyses ...
    
    // Also save to analysis_history
    historyID := uuid.New().String()
    historyQuery := `
        INSERT INTO analysis_history (id, developer_hash, analysis_id, score, confidence, input_type, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `
    _, err = s.db.Exec(historyQuery, historyID, developerHash, id, result.Score, result.Confidence, inputType, now)
    if err != nil {
        slog.Error("Failed to save analysis history", "error", err)
    }
    
    return nil
}
```

### 3. Hybrid Update Strategy

**File**: `backend/internal/leaderboard/service.go`

Add immediate update for top 10:

```go
// Add method for immediate top 10 update
func (s *Service) UpdateTop10Immediately(developerHash string, period string) error {
    // Calculate new weighted score
    weightedScore, avgConfidence, err := s.CalculateWeightedScore(developerHash)
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

func (s *Service) updateTop10ForPeriod(period string) error {
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
    
    // ... implementation to update top 10 ranks immediately ...
    // Invalidate top 10 cache entries
    s.cache.InvalidatePeriod(period)
    
    return nil
}
```

### 4. Opt-in API Endpoints

**File**: `backend/cmd/server/main.go`

Add new endpoints for opt-in workflow:

```go
// After analysis endpoint, add opt-in endpoint
r.POST("/leaderboard/opt-in", func(c *gin.Context) {
    var req struct {
        DeveloperHash string `json:"developer_hash" binding:"required"`
        OptIn         bool   `json:"opt_in" binding:"required"`
        DisplayName   string `json:"display_name"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Update opt-in status
    status := "declined"
    if req.OptIn {
        status = "accepted"
    }
    
    query := `
        UPDATE developer_analyses 
        SET leaderboard_opt_in_status = ?,
            leaderboard_opt_in_at = ?,
            display_name = ?,
            is_public = ?
        WHERE developer_hash = ?
    `
    
    _, err := db.Exec(query, status, time.Now(), req.DisplayName, req.OptIn, req.DeveloperHash)
    if err != nil {
        appLogger.APIErrorLogger(err, "POST", "/leaderboard/opt-in", c.ClientIP(), http.StatusInternalServerError)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update opt-in status"})
        return
    }
    
    // If opted in, trigger immediate top 10 update for all periods
    if req.OptIn {
        go func() {
            periods := []string{"daily", "weekly", "monthly", "all_time"}
            for _, period := range periods {
                if err := leaderboardService.UpdateTop10Immediately(req.DeveloperHash, period); err != nil {
                    slog.Error("Failed to update top 10 immediately", "period", period, "error", err)
                }
            }
        }()
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Opt-in status updated",
        "status":  status,
    })
})

// Modify /analyze endpoint to return developer_hash in response
// (around line 720)
response := gin.H{
    "score":          res.Score,
    "confidence":     res.Confidence,
    "posterior":      res.Posterior,
    "breakdown":      res.Breakdown,
    "contributors":   res.Contributors,
    "developer_hash": developerHash, // NEW: Include hash for opt-in modal
}
```

### 5. Display Name Support in Leaderboard Query

**File**: `backend/internal/leaderboard/service.go`

Update GetLeaderboard to include display names:

```go
// Modify LeaderboardEntry struct
type LeaderboardEntry struct {
    ID             string    `json:"id"`
    DeveloperHash  string    `json:"developer_hash"`
    DisplayName    *string   `json:"display_name,omitempty"` // NEW
    GitHubUsername *string   `json:"github_username,omitempty"` // NEW
    XUsername      *string   `json:"x_username,omitempty"` // NEW
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

// Update GetLeaderboard query to join with developer_analyses for display info
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

// Update scanning to include new fields
err := rows.Scan(
    &entry.ID, &entry.DeveloperHash, &entry.Period,
    &periodStartStr, &periodEndStr, &entry.Rank,
    &entry.Score, &entry.Confidence, &entry.InputType,
    &entry.IsPublic, &entry.CreatedAt,
    &entry.DisplayName, &entry.GitHubUsername, &entry.XUsername,
)
```

## Frontend Changes

### 6. Opt-in Modal Component

**File**: `frontend/src/components/LeaderboardOptInModal.tsx` (NEW)

Create modal component:

```tsx
import { createSignal, Show } from "solid-js";

interface LeaderboardOptInModalProps {
  isOpen: boolean;
  onClose: () => void;
  onOptIn: (optIn: boolean, displayName: string) => Promise<void>;
  developerHash: string;
  score: number;
}

export default function LeaderboardOptInModal(props: LeaderboardOptInModalProps) {
  const [displayName, setDisplayName] = createSignal("");
  const [isSubmitting, setIsSubmitting] = createSignal(false);
  const [useRealName, setUseRealName] = createSignal(true);

  const handleSubmit = async (optIn: boolean) => {
    setIsSubmitting(true);
    try {
      await props.onOptIn(optIn, useRealName() ? displayName() : "");
      props.onClose();
    } catch (error) {
      console.error("Failed to submit opt-in:", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="modal modal-open">
        <div class="modal-box max-w-2xl">
          <h3 class="font-bold text-2xl mb-4">🏆 Add to Leaderboard?</h3>
          
          <div class="bg-primary/10 rounded-lg p-4 mb-6">
            <div class="flex items-center justify-between">
              <div>
                <div class="text-sm text-base-content/70">Your Crackedness Score</div>
                <div class="text-4xl font-bold text-primary">{props.score.toFixed(1)}</div>
              </div>
              <div class="text-6xl">🎯</div>
            </div>
          </div>

          <p class="mb-4">
            Want to compete with other developers? Add your score to the public leaderboard!
          </p>

          <div class="form-control mb-6">
            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                class="checkbox"
                checked={useRealName()}
                onChange={(e) => setUseRealName(e.currentTarget.checked)}
              />
              <span class="label-text">
                Show my {props.githubUsername || props.xUsername ? "username" : "display name"}
              </span>
            </label>

            <Show when={useRealName()}>
              <div class="mt-3">
                <label class="label">
                  <span class="label-text">Display Name (optional)</span>
                </label>
                <input
                  type="text"
                  placeholder="Enter a display name or leave blank to use username"
                  class="input input-bordered w-full"
                  value={displayName()}
                  onInput={(e) => setDisplayName(e.currentTarget.value)}
                  maxLength={50}
                />
                <label class="label">
                  <span class="label-text-alt text-base-content/60">
                    Leave blank to show your GitHub/X username
                  </span>
                </label>
              </div>
            </Show>

            <Show when={!useRealName()}>
              <div class="alert alert-info mt-3">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
                <span>You'll appear as an anonymized developer ID</span>
              </div>
            </Show>
          </div>

          <div class="bg-base-200 rounded-lg p-4 mb-6">
            <h4 class="font-semibold mb-2">🔒 Privacy Information</h4>
            <ul class="text-sm space-y-1 text-base-content/70">
              <li>• Your analysis data is anonymized using SHA-256 hashing</li>
              <li>• Only scores with your consent appear on public leaderboards</li>
              <li>• You can remove your data at any time</li>
              <li>• Leaderboard data is retained for 90 days</li>
            </ul>
          </div>

          <div class="modal-action">
            <button
              class="btn btn-ghost"
              onClick={() => handleSubmit(false)}
              disabled={isSubmitting()}
            >
              No Thanks
            </button>
            <button
              class="btn btn-primary"
              onClick={() => handleSubmit(true)}
              disabled={isSubmitting()}
            >
              <Show when={isSubmitting()} fallback="Add to Leaderboard 🚀">
                <span class="loading loading-spinner"></span>
                Saving...
              </Show>
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
```

### 7. Update App.tsx for Modal Integration

**File**: `frontend/src/App.tsx`

Add modal integration (around line 27):

```tsx
const [showLeaderboardModal, setShowLeaderboardModal] = createSignal(false);
const [currentDeveloperHash, setCurrentDeveloperHash] = createSignal<string | null>(null);

// After analysis completes successfully, show opt-in modal
const [analysisResult, { refetch }] = createResource(trigger, async () => {
  // ... existing fetch logic ...
  
  const result = await response.json();
  
  // Store developer hash for opt-in
  if (result.developer_hash) {
    setCurrentDeveloperHash(result.developer_hash);
    // Show modal after short delay for better UX
    setTimeout(() => setShowLeaderboardModal(true), 1000);
  }
  
  return result;
});

// Add opt-in handler
const handleLeaderboardOptIn = async (optIn: boolean, displayName: string) => {
  if (!currentDeveloperHash()) return;
  
  const response = await fetch("/api/leaderboard/opt-in", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      developer_hash: currentDeveloperHash(),
      opt_in: optIn,
      display_name: displayName,
    }),
  });
  
  if (!response.ok) {
    throw new Error("Failed to update opt-in status");
  }
  
  setShowLeaderboardModal(false);
};

// Add modal to JSX (before closing </div>)
<LeaderboardOptInModal
  isOpen={showLeaderboardModal()}
  onClose={() => setShowLeaderboardModal(false)}
  onOptIn={handleLeaderboardOptIn}
  developerHash={currentDeveloperHash() || ""}
  score={analysisResult()?.score || 0}
/>
```

### 8. Top 10 Widget Component

**File**: `frontend/src/components/LeaderboardWidget.tsx` (NEW)

Create compact top 10 widget:

```tsx
import { createSignal, createResource, For, Show } from "solid-js";
import { LeaderboardEntry } from "../api";

interface LeaderboardWidgetProps {
  className?: string;
}

export default function LeaderboardWidget(props: LeaderboardWidgetProps) {
  const [selectedPeriod, setSelectedPeriod] = createSignal("weekly");

  const fetchTop10 = async (period: string): Promise<LeaderboardEntry[]> => {
    const response = await fetch(`/api/leaderboard/${period}?limit=10`);
    if (!response.ok) throw new Error("Failed to fetch top 10");
    const data = await response.json();
    return data.entries;
  };

  const [top10Data] = createResource(selectedPeriod, fetchTop10);

  const getDisplayName = (entry: LeaderboardEntry) => {
    if (entry.display_name) return entry.display_name;
    if (entry.github_username) return `@${entry.github_username}`;
    if (entry.x_username) return entry.x_username;
    return `Dev #${entry.developer_hash.substring(0, 6)}`;
  };

  const getRankMedal = (rank: number) => {
    if (rank === 1) return "🥇";
    if (rank === 2) return "🥈";
    if (rank === 3) return "🥉";
    return rank;
  };

  return (
    <div class={`card bg-gradient-to-br from-primary/5 to-secondary/5 shadow-xl ${props.className || ""}`}>
      <div class="card-body">
        <div class="flex items-center justify-between mb-4">
          <h3 class="card-title text-lg">🏆 Top 10 Developers</h3>
          <select
            class="select select-sm select-bordered"
            value={selectedPeriod()}
            onChange={(e) => setSelectedPeriod(e.currentTarget.value)}
          >
            <option value="daily">Today</option>
            <option value="weekly">This Week</option>
            <option value="monthly">This Month</option>
            <option value="all_time">All Time</option>
          </select>
        </div>

        <Show when={top10Data.loading}>
          <div class="flex justify-center py-4">
            <span class="loading loading-spinner loading-md"></span>
          </div>
        </Show>

        <Show when={!top10Data.loading && top10Data()}>
          <div class="space-y-2">
            <For each={top10Data()}>
              {(entry) => (
                <div class="flex items-center justify-between p-2 rounded-lg hover:bg-base-200 transition-colors">
                  <div class="flex items-center gap-3">
                    <div class="font-bold text-lg w-8 text-center">
                      {getRankMedal(entry.rank)}
                    </div>
                    <div>
                      <div class="font-medium text-sm">{getDisplayName(entry)}</div>
                      <div class="text-xs text-base-content/60">
                        {entry.input_type === "combined" ? "🔗" : entry.input_type === "github" ? "🐙" : "🐦"}
                      </div>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="font-bold text-primary">{entry.score.toFixed(1)}</div>
                    <div class="text-xs text-base-content/60">{(entry.confidence * 100).toFixed(0)}%</div>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>

        <div class="card-actions justify-end mt-4">
          <button
            class="btn btn-sm btn-ghost"
            onClick={() => {
              // Navigate to full leaderboard
              const event = new CustomEvent("navigate-leaderboard");
              window.dispatchEvent(event);
            }}
          >
            View Full Leaderboard →
          </button>
        </div>
      </div>
    </div>
  );
}
```

### 9. Update Leaderboard.tsx Display Names

**File**: `frontend/src/components/Leaderboard.tsx`

Update display logic (around line 330):

```tsx
const getDisplayName = (entry: LeaderboardEntry) => {
  // Priority: display_name > github_username > x_username > hash
  if (entry.display_name && entry.display_name.trim() !== "") {
    return entry.display_name;
  }
  if (entry.github_username) {
    return `@${entry.github_username}`;
  }
  if (entry.x_username) {
    return entry.x_username;
  }
  return `Developer #${entry.developer_hash.substring(0, 8)}`;
};

// Replace line 333:
<h4 class="font-semibold text-lg">
  {getDisplayName(entry)}
</h4>
```

### 10. Add Widget to Main Page

**File**: `frontend/src/App.tsx`

Add widget to main analysis page (around line 200, inside the main container):

```tsx
{/* Add after header, before input section */}
<Show when={activeTab() === "analysis"}>
  <div class="mb-8">
    <LeaderboardWidget className="max-w-md mx-auto" />
  </div>
</Show>
```

### 11. API Type Updates

**File**: `frontend/src/api.ts`

Update types:

```typescript
export interface LeaderboardEntry {
  id: string;
  developer_hash: string;
  display_name?: string; // NEW
  github_username?: string; // NEW
  x_username?: string; // NEW
  period: string;
  period_start: string;
  period_end: string;
  rank: number;
  score: number;
  confidence: number;
  input_type: string;
  is_public: boolean;
  created_at: string;
}

export interface AnalysisResult {
  score: number;
  confidence: number;
  posterior: number;
  breakdown: Record<string, number>;
  contributors: number;
  developer_hash?: string; // NEW
  user_stats?: UserStats;
}
```

## Testing & Validation

### Key Test Cases:

1. **Opt-in Modal**: Verify modal appears after analysis with correct score display
2. **Display Names**: Test display name vs username vs anonymized hash priority
3. **Weighted Scoring**: Verify multiple analyses calculate correct weighted average
4. **Hybrid Updates**: Confirm top 10 updates immediately, others batch update
5. **Widget Integration**: Ensure top 10 widget loads and updates correctly
6. **Privacy**: Verify only opted-in users appear on public leaderboard

## Migration Notes

- Existing `developer_analyses` entries need migration for new columns:
  - Set `leaderboard_opt_in_status = 'pending'` for existing entries
  - Set `display_name = NULL` for existing entries
- Create `analysis_history` records from existing `developer_analyses` data

## Performance Considerations

- Weighted score calculation cached per developer (15 min TTL)
- Top 10 queries use dedicated index on `score` and `rank`
- Batch updates run every 10 minutes for ranks 11-100
- Widget uses separate cache key from full leaderboard

### To-dos

- [ ] Update database schema with new columns (display_name, opt_in_status, opt_in_at) and analysis_history table
- [ ] Implement CalculateWeightedScore method with time decay, confidence, and type multipliers
- [ ] Implement hybrid update strategy (immediate top 10, batch for rest)
- [ ] Create POST /leaderboard/opt-in endpoint and modify /analyze to return developer_hash
- [ ] Update GetLeaderboard to join developer_analyses for display names and usernames
- [ ] Create LeaderboardOptInModal component with display name input and privacy info
- [ ] Integrate opt-in modal into App.tsx to show after analysis completion
- [ ] Create LeaderboardWidget component for compact top 10 display
- [ ] Add LeaderboardWidget to main analysis page in App.tsx
- [ ] Update Leaderboard.tsx to display names/usernames with proper fallback logic
- [ ] Update API types in api.ts for new LeaderboardEntry and AnalysisResult fields
- [ ] Test all components: modal workflow, display name priority, weighted scoring, hybrid updates, widget