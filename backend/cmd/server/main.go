package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/client"
	"github.com/stripe/stripe-go/v76/webhook"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/adapters"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/analysis"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/cache"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/database"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/encoding"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/errors"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/leaderboard"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/middleware"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/monitoring"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/privacy"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/resilience"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/security"
	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/types"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// Structured logging setup
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Configuration from environment with defaults
	dataDir := getEnvOrDefault("DATA_DIR", "./data")
	githubToken := os.Getenv("GITHUB_TOKEN")
	xBearerToken := os.Getenv("X_BEARER_TOKEN")
	jwtSecret := getEnvOrDefault("JWT_SECRET", "your-super-secret-jwt-key-change-in-production")
	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	port := getEnvOrDefault("PORT", "8080")

	// Initialize database and user service
	db, err := database.NewDB(dataDir)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := database.NewRepository(db)
	userService := database.NewUserService(repo, jwtSecret)

	// Initialize leaderboard service
	leaderboardService := leaderboard.NewService(db)

	// Initialize privacy service
	privacyService := privacy.NewService(db)

	// Initialize optimized JSON encoder
	optimizedEncoder := encoding.NewOptimizedJSONEncoder()

	// Initialize compression middleware
	compressionConfig := middleware.DefaultCompressionConfig()
	compressionMiddleware := middleware.NewCompressionMiddleware(compressionConfig)

	// Warm up leaderboard cache and start auto-refresh
	go func() {
		slog.Info("Warming up leaderboard cache")
		leaderboardService.WarmCache()
		leaderboardService.StartAutoRefresh(10 * time.Minute) // Refresh every 10 minutes
	}()

	// Schedule data cleanup (runs daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := privacyService.ScheduleDataCleanup(365); err != nil {
				slog.Error("Failed to schedule data cleanup", "error", err)
			}
		}
	}()

	// Initialize Stripe client
	var stripeClient *client.API
	if stripeSecretKey != "" {
		stripe.Key = stripeSecretKey
		stripeClient = &client.API{}
		stripeClient.Init(stripeSecretKey, nil)
	}

	// Create analyzer and adapters
	analyzer := analysis.NewAnalyzer(dataDir)
	githubAdapter := adapters.NewGitHubAdapter(githubToken)
	xAdapter := adapters.NewXAdapterWithToken(xBearerToken)

	r := gin.New()

	// Initialize monitoring system
	appMetrics := monitoring.NewMetrics()
	appLogger := monitoring.NewLogger()

	// Initialize memory monitor
	memoryMonitor := monitoring.NewMemoryMonitor(5*time.Second, 50*1024*1024, appLogger) // 50MB GC threshold
	memoryMonitor.Start()

	// Add compression middleware
	r.Use(gin.HandlerFunc(func(c *gin.Context) {
		compressionMiddleware.Handler()(c.Writer, c.Request)
	}))

	// Initialize distributed tracing
	monitoring.InitGlobalTracer("cracked-dev-o-meter", appLogger)

	// Initialize alerting system
	monitoring.InitGlobalAlertManager(appLogger, 30*time.Second)

	// Add Slack notifier (configure webhook URL in production)
	slackNotifier := monitoring.NewSlackNotifier(os.Getenv("SLACK_WEBHOOK_URL"))
	if slackNotifier.WebhookURL != "" {
		alertManager := monitoring.GetGlobalAlertManager()
		alertManager.AddNotifier(slackNotifier)
	}

	// Start alerting in background
	monitoring.StartGlobalAlerting(context.Background())

	// Add monitoring middleware first (to capture all requests)
	r.Use(monitoring.MonitoringMiddleware(appMetrics, appLogger))
	r.Use(monitoring.TracingMiddleware(monitoring.GetGlobalTracer()))
	r.Use(monitoring.SecurityMonitoringMiddleware(appLogger))

	// Add error handling middleware
	r.Use(errors.ErrorHandler())
	r.Use(errors.RecoveryHandler())

	// Security middleware setup
	securityConfig := security.DefaultSecurityConfig()
	securityMiddleware := security.NewSecurityMiddleware(securityConfig)
	securityMiddleware.SetUserService(userService)

	// Add security middleware
	r.Use(securityMiddleware.SecurityHeaders)
	r.Use(securityMiddleware.RequestTimeout)
	r.Use(securityMiddleware.ValidateContentType)
	r.Use(securityMiddleware.RateLimitByIP)
	r.Use(securityMiddleware.UserRateLimit)

	// Initialize cache (15 minutes TTL)
	appCache := cache.NewCache(15 * time.Minute)
	r.Use(appCache.Middleware(appMetrics))

	// Register external services for degradation management
	resilience.RegisterService("github-api", func(ctx context.Context) error {
		// Simple health check - in production this would be a real health check
		return nil // Assume healthy for now
	})

	resilience.RegisterService("x-api", func(ctx context.Context) error {
		// Simple health check - in production this would be a real health check
		return nil // Assume healthy for now
	})

	// Start health checks in background
	resilience.StartHealthChecks(context.Background())

	r.GET("/health", func(c *gin.Context) {
		// Get service health status
		services := resilience.GetAllServiceHealth()

		// Get system metrics
		metrics := appMetrics.GetStats()

		healthResponse := gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
			"services":  services,
			"metrics":   metrics,
		}

		// Check if any service is in emergency state
		for _, service := range services {
			if service.Level == resilience.LevelEmergency {
				healthResponse["status"] = "degraded"
				c.JSON(http.StatusServiceUnavailable, healthResponse)
				return
			}
		}

		c.JSON(http.StatusOK, healthResponse)
	})

	// Service health and circuit breaker monitoring endpoint
	r.GET("/health/services", func(c *gin.Context) {
		services := resilience.GetAllServiceHealth()
		circuitStats := resilience.GetCircuitBreakerStats()
		alerts := monitoring.GetGlobalAlertManager().GetActiveAlerts()

		response := gin.H{
			"services":         services,
			"circuit_breakers": circuitStats,
			"active_alerts":    alerts,
			"timestamp":        time.Now().Format(time.RFC3339),
		}

		c.JSON(http.StatusOK, response)
	})

	// Tracing endpoint to get current traces
	r.GET("/debug/traces", func(c *gin.Context) {
		tracer := monitoring.GetGlobalTracer()
		if tracer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tracing not initialized"})
			return
		}

		traces := make(map[string]interface{})
		for spanID, span := range tracer.GetSpans() {
			traces[string(spanID)] = span
		}

		c.JSON(http.StatusOK, gin.H{
			"traces":    traces,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Alerting endpoints
	r.GET("/alerts", func(c *gin.Context) {
		alerts := monitoring.GetGlobalAlertManager().GetAlerts()
		c.JSON(http.StatusOK, gin.H{
			"alerts":    alerts,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	r.POST("/alerts/:id/silence", func(c *gin.Context) {
		alertID := c.Param("id")
		duration := 30 * time.Minute // Default 30 minutes

		if durationParam := c.Query("duration"); durationParam != "" {
			if d, err := time.ParseDuration(durationParam); err == nil {
				duration = d
			}
		}

		monitoring.GetGlobalAlertManager().SilenceAlert(alertID, duration)

		c.JSON(http.StatusOK, gin.H{
			"message":  "Alert silenced",
			"alert_id": alertID,
			"duration": duration.String(),
		})
	})

	// User stats endpoint
	r.GET("/user/stats", func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not identified"})
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
			return
		}

		stats, err := userService.GetUserStats(userIDStr)
		if err != nil {
			appErr := errors.ToAppError(err)
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		c.JSON(http.StatusOK, stats)
	})

	// Create Stripe checkout session
	r.POST("/payment/create-session", func(c *gin.Context) {
		if stripeClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment system not configured"})
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not identified"})
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
			return
		}

		var req struct {
			Type   string `json:"type" binding:"required"` // "donation" or "unlimited"
			Amount int64  `json:"amount,omitempty"`        // For donations
		}

		if err := c.BindJSON(&req); err != nil {
			appErr := errors.ToAppError(err)
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		var priceID string
		var paymentType string

		if req.Type == "unlimited" {
			// Monthly unlimited access - you'll need to create this price in Stripe
			priceID = "price_unlimited_monthly" // Replace with actual Stripe price ID
			paymentType = "subscription"
		} else if req.Type == "donation" && req.Amount > 0 {
			// One-time donation
			paymentType = "donation"
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment type or amount"})
			return
		}

		var sessionParams *stripe.CheckoutSessionParams

		if paymentType == "subscription" {
			sessionParams = &stripe.CheckoutSessionParams{
				PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
				LineItems: []*stripe.CheckoutSessionLineItemParams{
					{
						Price:    stripe.String(priceID),
						Quantity: stripe.Int64(1),
					},
				},
				Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
				SuccessURL:        stripe.String("https://your-domain.com/payment/success?session_id={CHECKOUT_SESSION_ID}"),
				CancelURL:         stripe.String("https://your-domain.com/payment/cancelled"),
				ClientReferenceID: stripe.String(userIDStr),
				Metadata: map[string]string{
					"user_id": userIDStr,
					"type":    paymentType,
				},
			}
		} else {
			// One-time donation
			sessionParams = &stripe.CheckoutSessionParams{
				PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
				LineItems: []*stripe.CheckoutSessionLineItemParams{
					{
						PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
							Currency: stripe.String("usd"),
							ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
								Name:        stripe.String("Donation to Cracked Dev-o-Meter"),
								Description: stripe.String("Support for maintaining and improving the developer analysis tool"),
							},
							UnitAmount: stripe.Int64(req.Amount * 100), // Convert to cents
						},
						Quantity: stripe.Int64(1),
					},
				},
				Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
				SuccessURL:        stripe.String("https://your-domain.com/payment/success?session_id={CHECKOUT_SESSION_ID}"),
				CancelURL:         stripe.String("https://your-domain.com/payment/cancelled"),
				ClientReferenceID: stripe.String(userIDStr),
				Metadata: map[string]string{
					"user_id": userIDStr,
					"type":    paymentType,
					"amount":  fmt.Sprintf("%d", req.Amount),
				},
			}
		}

		session, err := stripeClient.CheckoutSessions.New(sessionParams)
		if err != nil {
			appErr := errors.ToAppError(err)
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"session_id": session.ID,
			"url":        session.URL,
		})
	})

	// Stripe webhook endpoint
	r.POST("/payment/webhook", func(c *gin.Context) {
		if stripeClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment system not configured"})
			return
		}

		const MaxBodyBytes = int64(65536)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)

		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		// Verify webhook signature (you'll need to set up webhook endpoint secret)
		endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
		if endpointSecret != "" {
			// Webhook signature verification would go here
			// This is a simplified version - in production you'd verify the signature
		}

		event, err := webhook.ConstructEvent(body, c.GetHeader("Stripe-Signature"), endpointSecret)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse webhook"})
			return
		}

		switch event.Type {
		case "checkout.session.completed":
			var session stripe.CheckoutSession
			err := json.Unmarshal(event.Data.Raw, &session)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse session"})
				return
			}

			userID := session.ClientReferenceID
			paymentType := session.Metadata["type"]

			if userID == "" {
				slog.Error("User ID is empty in webhook")
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
				return
			}

			switch paymentType {
			case "subscription":
				// Upgrade user to paid
				err = userService.UpgradeUserToPaid(userID, session.Customer.ID)
				if err != nil {
					slog.Error("Failed to upgrade user", "error", err, "user_id", userID)
				}
			case "donation":
				// Record donation payment
				amount := session.AmountTotal / 100 // Convert from cents
				_, err = userService.CreatePaymentRecord(userID, session.ID, string(session.Currency), "completed", paymentType, int64(amount))
				if err != nil {
					slog.Error("Failed to record donation", "error", err, "user_id", userID)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	r.POST("/analyze", func(c *gin.Context) {
		// Add timeout context
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		var req types.AnalyzeRequest
		if err := c.BindJSON(&req); err != nil {
			appErr := errors.ToAppError(err)
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		// Sanitize input
		req.Input = strings.TrimSpace(req.Input)
		if req.Input == "" {
			appErr := errors.NewValidationError("input cannot be empty")
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		slog.Info("Starting analysis", "input", req.Input, "ip", c.ClientIP())

		// Parse input for GitHub and X usernames
		githubUsername, xUsername := parseCombinedInput(req.Input)

		var githubEvents []types.RawEvent
		var xEvents []types.RawEvent

		// Fetch GitHub data if username provided
		if githubUsername != "" {
			// Check if GitHub service is available
			if !resilience.IsServiceAvailable("github-api") {
				slog.Warn("GitHub service is unavailable due to high error rate", "username", githubUsername)
				// Continue without GitHub data
			} else {
				var ghEvents []adapters.GitHubEvent

				// Use circuit breaker and retry for GitHub API calls
				err := resilience.ExecuteWithRetry(ctx, "github-api", func() error {
					if strings.Contains(githubUsername, "/") {
						// It's a repository
						parts := strings.Split(githubUsername, "/")
						if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
							var err error
							ghEvents, err = githubAdapter.FetchRepoData(ctx, parts[0], parts[1])
							return err
						} else {
							return errors.NewValidationError("invalid repository format (use owner/repo)")
						}
					} else {
						// It's a username
						var err error
						ghEvents, err = githubAdapter.FetchUserData(ctx, githubUsername)
						return err
					}
				})

				if err != nil {
					slog.Error("GitHub API error", "error", err, "username", githubUsername)
					resilience.RecordError("github-api", err)
					appMetrics.IncrementGitHubCalls()
					appLogger.ExternalAPILogger("GitHub", "GET", "api.github.com", 500, 0, false)
					// Continue without GitHub data rather than failing completely
					slog.Warn("Continuing analysis without GitHub data", "ip", c.ClientIP())
				} else {
					resilience.RecordRequest("github-api", true)
					appMetrics.IncrementGitHubCalls()
					appLogger.ExternalAPILogger("GitHub", "GET", "api.github.com", 200, 0, true)
					// Convert GitHub events to RawEvents
					githubEvents = make([]types.RawEvent, len(ghEvents))
					for i, gh := range ghEvents {
						githubEvents[i] = types.RawEvent{
							Type:      gh.Type,
							Timestamp: time.Now(),
							Count:     gh.Count,
							Repo:      gh.Repo,
							Language:  gh.Language,
						}
					}
				}
			}
		}

		// Fetch X data if username provided and adapter is authenticated
		if xUsername != "" && xAdapter.IsAuthenticated() {
			// Check if X service is available
			if !resilience.IsServiceAvailable("x-api") {
				slog.Warn("X service is unavailable due to high error rate", "username", xUsername)
				// Continue without X data
			} else {
				var xAdapterEvents []adapters.XEvent

				// Use circuit breaker and retry for X API calls
				err := resilience.ExecuteWithRetry(ctx, "x-api", func() error {
					var err error
					xAdapterEvents, err = xAdapter.FetchUserData(ctx, xUsername)
					return err
				})

				if err != nil {
					slog.Error("X API error", "error", err, "username", xUsername)
					resilience.RecordError("x-api", err)
					appMetrics.IncrementXCalls()
					appLogger.ExternalAPILogger("X", "GET", "api.twitter.com", 500, 0, false)
					// Continue without X data rather than failing completely
					slog.Warn("Continuing analysis without X data", "ip", c.ClientIP())
				} else {
					resilience.RecordRequest("x-api", true)
					appMetrics.IncrementXCalls()
					appLogger.ExternalAPILogger("X", "GET", "api.twitter.com", 200, 0, true)
					xEvents = convertXEventsToRawEvents(xAdapterEvents)
				}
			}
		} else if xUsername != "" && !xAdapter.IsAuthenticated() {
			slog.Warn("X analysis requested but no bearer token configured", "username", xUsername, "ip", c.ClientIP())
		}

		// Perform analysis based on available data
		var res analysis.ScoreResult
		var err error

		if len(githubEvents) > 0 && len(xEvents) > 0 {
			// Combined GitHub + X analysis
			slog.Info("Performing combined GitHub + X analysis",
				"github_events", len(githubEvents),
				"x_events", len(xEvents),
				"github_user", githubUsername,
				"x_user", xUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEventsWithX(githubEvents, xEvents, req.Input)
		} else if len(githubEvents) > 0 {
			// GitHub-only analysis
			slog.Info("Performing GitHub-only analysis",
				"events", len(githubEvents),
				"user", githubUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEvents(githubEvents, req.Input)
		} else if len(xEvents) > 0 {
			// X-only analysis
			slog.Info("Performing X-only analysis",
				"events", len(xEvents),
				"user", xUsername,
				"ip", c.ClientIP())
			res, err = analyzer.AnalyzeEvents(xEvents, req.Input)
		} else {
			slog.Warn("No analyzable data found", "input", req.Input, "ip", c.ClientIP())
			appErr := errors.NewValidationError("no analyzable data found for the provided input")
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		if err != nil {
			slog.Error("Analysis failed", "error", err, "input", req.Input)
			appErr := errors.ToAppError(err)
			errors.LogError(c, appErr)
			c.JSON(appErr.HTTPStatus, appErr)
			return
		}

		slog.Info("Analysis completed", "input", req.Input, "score", res.Score, "confidence", res.Confidence)

		// Enhanced analysis logging with performance metrics
		cacheHit := c.GetBool("cache_hit")
		analysisStart := c.GetTime("analysis_start")
		if !analysisStart.IsZero() {
			analysisDuration := time.Since(analysisStart)
			appLogger.AnalysisLogger(req.Input, getAnalysisType(githubEvents, xEvents), float64(res.Score), res.Confidence, analysisDuration, cacheHit)
		}

		// Save analysis to leaderboard (async to avoid blocking response)
		go func() {
			inputType := getAnalysisType(githubEvents, xEvents)
			ipAddress := c.ClientIP()
			userAgent := c.GetHeader("User-Agent")
			isPublic := c.Query("public") == "true" // Allow users to opt-in to public leaderboard

			// Check privacy consent
			hasConsent := privacyService.ValidatePrivacyConsent(req.Input, inputType, isPublic)

			if hasConsent {
				err := leaderboardService.SaveAnalysis(res, req.Input, inputType, ipAddress, userAgent, &githubUsername, &xUsername, isPublic)
				if err != nil {
					slog.Error("Failed to save analysis to leaderboard", "error", err, "input", req.Input)
				} else {
					slog.Info("Analysis saved to leaderboard with privacy consent", "input_type", inputType, "is_public", isPublic)
				}
			} else {
				slog.Info("Analysis not saved to leaderboard - no privacy consent", "input_type", inputType, "is_public", isPublic)
			}
		}()

		// Include user statistics in response
		userID, hasUserID := c.Get("user_id")
		response := gin.H{
			"score":        res.Score,
			"confidence":   res.Confidence,
			"posterior":    res.Posterior,
			"breakdown":    res.Breakdown,
			"contributors": res.Contributors,
		}

		if hasUserID {
			userIDStr, ok := userID.(string)
			if ok {
				userStats, err := userService.GetUserStats(userIDStr)
				if err == nil {
					response["user_stats"] = userStats
				}
			}
		}

		c.JSON(http.StatusOK, response)
	})

	// Swagger documentation routes
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Metrics endpoint
	r.GET("/metrics", func(c *gin.Context) {
		stats := appMetrics.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	// Cache stats endpoint
	r.GET("/cache/stats", func(c *gin.Context) {
		stats := appCache.Stats()
		c.JSON(http.StatusOK, stats)
	})

	// Leaderboard cache stats endpoint
	r.GET("/leaderboard/cache/stats", func(c *gin.Context) {
		stats := leaderboardService.GetCacheStats()
		c.JSON(http.StatusOK, stats)
	})

	// Connection pool stats endpoints
	r.GET("/pools/github", func(c *gin.Context) {
		stats := githubAdapter.GetPoolStats()
		c.JSON(http.StatusOK, gin.H{
			"pool":  "github",
			"stats": stats,
		})
	})

	r.GET("/pools/x", func(c *gin.Context) {
		stats := xAdapter.GetPoolStats()
		c.JSON(http.StatusOK, gin.H{
			"pool":  "x",
			"stats": stats,
		})
	})

	// Database pool stats endpoint
	r.GET("/pools/database", func(c *gin.Context) {
		stats := db.GetPoolStats()
		c.JSON(http.StatusOK, gin.H{
			"pool":  "database",
			"stats": stats,
		})
	})

	// JSON encoder stats endpoint
	r.GET("/pools/json", func(c *gin.Context) {
		stats := optimizedEncoder.GetStats()
		c.JSON(http.StatusOK, gin.H{
			"pool":  "json",
			"stats": stats,
		})
	})

	// Compression stats endpoint
	r.GET("/pools/compression", func(c *gin.Context) {
		stats := compressionMiddleware.GetStats()
		c.JSON(http.StatusOK, gin.H{
			"pool":  "compression",
			"stats": stats,
		})
	})

	// Memory stats endpoint
	r.GET("/memory", func(c *gin.Context) {
		stats := memoryMonitor.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	// Memory optimization endpoint
	r.POST("/memory/optimize", func(c *gin.Context) {
		memoryMonitor.OptimizeMemory()
		c.JSON(http.StatusOK, gin.H{"message": "memory optimization triggered"})
	})

	// Force GC endpoint (development only)
	if os.Getenv("ENABLE_GC_CONTROL") == "true" {
		r.POST("/memory/gc", func(c *gin.Context) {
			memoryMonitor.ForceGC()
			c.JSON(http.StatusOK, gin.H{"message": "garbage collection triggered"})
		})
	}

	// Performance profiling endpoints (development only)
	if os.Getenv("ENABLE_PROFILING") == "true" {
		slog.Info("Enabling performance profiling endpoints")
		// Mount pprof endpoints
		r.GET("/debug/pprof/*filepath", gin.WrapF(pprof.Index))
		r.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
		r.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
		r.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
		r.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	}

	// Leaderboard endpoints
	r.GET("/leaderboard/:period", func(c *gin.Context) {
		period := c.Param("period")
		limit := 50

		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		response, err := leaderboardService.GetLeaderboard(period, limit)
		if err != nil {
			appLogger.APIErrorLogger(err, "GET", "/leaderboard/"+period, c.ClientIP(), http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve leaderboard"})
			return
		}

		c.JSON(http.StatusOK, response)
	})

	r.GET("/leaderboard/:period/rank/:hash", func(c *gin.Context) {
		period := c.Param("period")
		hash := c.Param("hash")

		entry, err := leaderboardService.GetDeveloperRank(hash, period)
		if err != nil {
			appLogger.APIErrorLogger(err, "GET", "/leaderboard/"+period+"/rank/"+hash, c.ClientIP(), http.StatusNotFound)
			c.JSON(http.StatusNotFound, gin.H{"error": "rank not found"})
			return
		}

		c.JSON(http.StatusOK, entry)
	})

	r.POST("/leaderboard/update", func(c *gin.Context) {
		// This endpoint would be called by a scheduled job or admin
		// In production, this should be protected by authentication
		if err := leaderboardService.UpdateLeaderboards(); err != nil {
			appLogger.APIErrorLogger(err, "POST", "/leaderboard/update", c.ClientIP(), http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update leaderboards"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "leaderboards updated successfully"})
	})

	// Privacy endpoints
	r.GET("/privacy/policy", func(c *gin.Context) {
		policy := privacyService.GetDataRetentionInfo()
		c.JSON(http.StatusOK, policy)
	})

	r.GET("/privacy/settings/:hash", func(c *gin.Context) {
		developerHash := c.Param("hash")
		settings, err := privacyService.GetPrivacySettings(developerHash)
		if err != nil {
			appLogger.APIErrorLogger(err, "GET", "/privacy/settings/"+developerHash, c.ClientIP(), http.StatusNotFound)
			c.JSON(http.StatusNotFound, gin.H{"error": "privacy settings not found"})
			return
		}

		c.JSON(http.StatusOK, settings)
	})

	r.POST("/privacy/delete/:hash", func(c *gin.Context) {
		developerHash := c.Param("hash")

		// In production, add authentication and confirmation requirements
		if err := privacyService.DeleteUserData(developerHash); err != nil {
			appLogger.APIErrorLogger(err, "POST", "/privacy/delete/"+developerHash, c.ClientIP(), http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "user data deleted successfully",
			"developer_hash": developerHash[:8] + "...",
		})
	})

	r.PUT("/privacy/settings/:hash", func(c *gin.Context) {
		developerHash := c.Param("hash")

		var requestBody struct {
			IsPublic bool `json:"is_public" binding:"required"`
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if err := privacyService.UpdatePrivacySettings(developerHash, requestBody.IsPublic); err != nil {
			appLogger.APIErrorLogger(err, "PUT", "/privacy/settings/"+developerHash, c.ClientIP(), http.StatusInternalServerError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update privacy settings"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "privacy settings updated successfully",
			"developer_hash": developerHash[:8] + "...",
			"is_public":      requestBody.IsPublic,
		})
	})

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		slog.Info("Starting server", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close adapter connection pools
	githubAdapter.Close()
	xAdapter.Close()

	// Stop memory monitor
	memoryMonitor.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited")
}

// Helper function for environment variables with defaults
// parseCombinedInput parses input that may contain both GitHub and X usernames
// Supports formats like:
// - "github:torvalds x:elonmusk"
// - "torvalds (github) @elonmusk (x)"
// - "github:torvalds"
// - "@elonmusk"
// - "torvalds" (assumes GitHub username)
func parseCombinedInput(input string) (githubUsername, xUsername string) {
	input = strings.TrimSpace(input)

	// Check for explicit GitHub/X format
	if strings.Contains(input, "github:") && strings.Contains(input, "x:") {
		// Parse "github:username x:username" format
		githubMatch := strings.Split(input, "github:")
		if len(githubMatch) > 1 {
			githubPart := strings.TrimSpace(strings.Split(githubMatch[1], " ")[0])
			githubUsername = strings.TrimPrefix(githubPart, "@")
		}

		xMatch := strings.Split(input, "x:")
		if len(xMatch) > 1 {
			xPart := strings.TrimSpace(strings.Split(xMatch[1], " ")[0])
			xUsername = strings.TrimPrefix(xPart, "@")
		}
		return
	}

	// Check for GitHub-only format
	if strings.HasPrefix(input, "github:") {
		githubUsername = strings.TrimSpace(strings.TrimPrefix(input, "github:"))
		githubUsername = strings.TrimPrefix(githubUsername, "@")
		return
	}

	// Check for X-only format
	if strings.HasPrefix(input, "x:") || strings.HasPrefix(input, "@") {
		xUsername = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(input, "x:"), "@"))
		return
	}

	// Default: assume GitHub username
	githubUsername = input
	return
}

// convertXEventsToRawEvents converts X adapter events to RawEvent format
func convertXEventsToRawEvents(xEvents []adapters.XEvent) []types.RawEvent {
	rawEvents := make([]types.RawEvent, len(xEvents))
	for i, xEvent := range xEvents {
		rawEvents[i] = types.RawEvent{
			Type:      xEvent.Type,
			Timestamp: time.Now(),
			Count:     xEvent.Count,
			Repo:      xEvent.Handle, // Use Handle as Repo for consistency
		}
	}
	return rawEvents
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getAnalysisType determines the type of analysis performed based on available data
func getAnalysisType(githubEvents, xEvents []types.RawEvent) string {
	hasGitHub := len(githubEvents) > 0
	hasX := len(xEvents) > 0

	switch {
	case hasGitHub && hasX:
		return "combined_github_x"
	case hasGitHub:
		return "github_only"
	case hasX:
		return "x_only"
	default:
		return "no_data"
	}
}
