package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityError    AlertSeverity = "error"
	SeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	StatusActive     AlertStatus = "active"
	StatusResolved   AlertStatus = "resolved"
	StatusSuppressed AlertStatus = "suppressed"
)

// Alert represents a monitoring alert
type Alert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Service     string            `json:"service"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Value       float64           `json:"value,omitempty"`
	Threshold   float64           `json:"threshold,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	FiredAt     time.Time         `json:"fired_at"`
	LastSentAt  *time.Time        `json:"last_sent_at,omitempty"`
}

// AlertRule defines a rule for generating alerts
type AlertRule struct {
	Name        string
	Query       string  // Metric query or condition
	Threshold   float64 // Threshold value
	Operator    string  // "gt", "lt", "eq", "ne", "gte", "lte"
	Severity    AlertSeverity
	Service     string
	Description string
	Labels      map[string]string
	Annotations map[string]string
	For         time.Duration // Time condition must be true before firing
}

// AlertNotifier defines the interface for sending alert notifications
type AlertNotifier interface {
	SendAlert(ctx context.Context, alert *Alert) error
	ResolveAlert(ctx context.Context, alert *Alert) error
}

// SlackNotifier sends alerts to Slack
type SlackNotifier struct {
	WebhookURL string
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
	}
}

// SendAlert sends an alert to Slack
func (s *SlackNotifier) SendAlert(ctx context.Context, alert *Alert) error {
	// Implementation would send HTTP request to Slack webhook
	// Simplified for this example
	slog.Info("Slack alert sent", "alert", alert.Name, "severity", alert.Severity)
	return nil
}

// ResolveAlert resolves an alert in Slack
func (s *SlackNotifier) ResolveAlert(ctx context.Context, alert *Alert) error {
	// Implementation would send resolution notification to Slack
	slog.Info("Slack alert resolved", "alert", alert.Name)
	return nil
}

// EmailNotifier sends alerts via email
type EmailNotifier struct {
	SMTPHost  string
	SMTPPort  int
	Username  string
	Password  string
	FromEmail string
	ToEmails  []string
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(smtpHost string, smtpPort int, username, password, fromEmail string, toEmails []string) *EmailNotifier {
	return &EmailNotifier{
		SMTPHost:  smtpHost,
		SMTPPort:  smtpPort,
		Username:  username,
		Password:  password,
		FromEmail: fromEmail,
		ToEmails:  toEmails,
	}
}

// SendAlert sends an alert via email
func (e *EmailNotifier) SendAlert(ctx context.Context, alert *Alert) error {
	// Implementation would send email
	// Simplified for this example
	slog.Info("Email alert sent", "alert", alert.Name, "to", e.ToEmails)
	return nil
}

// ResolveAlert resolves an alert via email
func (e *EmailNotifier) ResolveAlert(ctx context.Context, alert *Alert) error {
	// Implementation would send resolution email
	slog.Info("Email alert resolved", "alert", alert.Name)
	return nil
}

// AlertManager manages alerts and notifications
type AlertManager struct {
	rules         []AlertRule
	alerts        map[string]*Alert
	notifiers     []AlertNotifier
	logger        *Logger
	checkInterval time.Duration
}

// NewAlertManager creates a new alert manager
func NewAlertManager(logger *Logger, checkInterval time.Duration) *AlertManager {
	return &AlertManager{
		rules:         []AlertRule{},
		alerts:        make(map[string]*Alert),
		notifiers:     []AlertNotifier{},
		logger:        logger,
		checkInterval: checkInterval,
	}
}

// AddRule adds an alert rule
func (am *AlertManager) AddRule(rule AlertRule) {
	am.rules = append(am.rules, rule)
}

// AddNotifier adds a notifier
func (am *AlertManager) AddNotifier(notifier AlertNotifier) {
	am.notifiers = append(am.notifiers, notifier)
}

// Start begins the alert evaluation loop
func (am *AlertManager) Start(ctx context.Context) {
	ticker := time.NewTicker(am.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			am.evaluateRules(ctx)
		}
	}
}

// evaluateRules evaluates all alert rules
func (am *AlertManager) evaluateRules(ctx context.Context) {
	for _, rule := range am.rules {
		am.evaluateRule(ctx, rule)
	}
}

// evaluateRule evaluates a single alert rule
func (am *AlertManager) evaluateRule(ctx context.Context, rule AlertRule) {
	// Simplified evaluation - in practice, this would query metrics
	// For this example, we'll simulate some metric evaluation

	var currentValue float64
	switch rule.Query {
	case "error_rate":
		// Get current error rate from metrics
		// This is a simplified example
		currentValue = am.getCurrentErrorRate(rule.Service)
	case "response_time":
		currentValue = am.getCurrentResponseTime(rule.Service)
	case "memory_usage":
		currentValue = am.getCurrentMemoryUsage()
	case "cpu_usage":
		currentValue = am.getCurrentCPUUsage()
	default:
		am.logger.SystemLogger("unknown_alert_query", fmt.Sprintf("Unknown query type: %s", rule.Query))
		return
	}

	alertKey := fmt.Sprintf("%s:%s", rule.Service, rule.Name)
	alert, exists := am.alerts[alertKey]

	// Check if condition is met
	conditionMet := am.checkCondition(currentValue, rule.Operator, rule.Threshold)

	if conditionMet {
		if !exists {
			// Create new alert
			alert = &Alert{
				ID:          alertKey,
				Name:        rule.Name,
				Description: rule.Description,
				Severity:    rule.Severity,
				Status:      StatusActive,
				Service:     rule.Service,
				Labels:      rule.Labels,
				Annotations: rule.Annotations,
				Value:       currentValue,
				Threshold:   rule.Threshold,
				CreatedAt:   time.Now(),
				FiredAt:     time.Now(),
			}
			am.alerts[alertKey] = alert
			am.fireAlert(ctx, alert)
		} else if exists && alert.Status != StatusActive {
			// Re-fire existing alert
			alert.Status = StatusActive
			alert.FiredAt = time.Now()
			alert.Value = currentValue
			am.fireAlert(ctx, alert)
		}
	} else if exists && alert.Status == StatusActive {
		// Check if alert should be resolved
		if time.Since(alert.FiredAt) > rule.For {
			alert.Status = StatusResolved
			alert.ResolvedAt = &time.Time{}
			*alert.ResolvedAt = time.Now()
			am.resolveAlert(ctx, alert)
		}
	}
}

// checkCondition checks if a condition is met
func (am *AlertManager) checkCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "lt":
		return value < threshold
	case "eq":
		return value == threshold
	case "ne":
		return value != threshold
	case "gte":
		return value >= threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

// fireAlert fires an alert to all notifiers
func (am *AlertManager) fireAlert(ctx context.Context, alert *Alert) {
	am.logger.SystemLogger("alert_fired", fmt.Sprintf("Alert %s fired with severity %s", alert.Name, alert.Severity))

	for _, notifier := range am.notifiers {
		go func(n AlertNotifier) {
			if err := n.SendAlert(ctx, alert); err != nil {
				am.logger.SystemLogger("alert_notification_failed", fmt.Sprintf("Failed to send alert %s: %v", alert.Name, err))
			}
		}(notifier)
	}
}

// resolveAlert resolves an alert with all notifiers
func (am *AlertManager) resolveAlert(ctx context.Context, alert *Alert) {
	am.logger.SystemLogger("alert_resolved", fmt.Sprintf("Alert %s resolved", alert.Name))

	for _, notifier := range am.notifiers {
		go func(n AlertNotifier) {
			if err := n.ResolveAlert(ctx, alert); err != nil {
				am.logger.SystemLogger("alert_resolution_failed", fmt.Sprintf("Failed to resolve alert %s: %v", alert.Name, err))
			}
		}(notifier)
	}
}

// GetAlerts returns all current alerts
func (am *AlertManager) GetAlerts() map[string]*Alert {
	alerts := make(map[string]*Alert)
	for k, v := range am.alerts {
		alerts[k] = v
	}
	return alerts
}

// GetActiveAlerts returns only active alerts
func (am *AlertManager) GetActiveAlerts() map[string]*Alert {
	activeAlerts := make(map[string]*Alert)
	for k, v := range am.alerts {
		if v.Status == StatusActive {
			activeAlerts[k] = v
		}
	}
	return activeAlerts
}

// SilenceAlert silences an alert
func (am *AlertManager) SilenceAlert(alertID string, duration time.Duration) {
	if alert, exists := am.alerts[alertID]; exists {
		alert.Status = StatusSuppressed
		// In a real implementation, you'd store the silence duration
		am.logger.SystemLogger("alert_silenced", fmt.Sprintf("Alert %s silenced for %v", alert.Name, duration))
	}
}

// Simulate metric getters (in a real implementation, these would query actual metrics)
func (am *AlertManager) getCurrentErrorRate(service string) float64 {
	// Simplified - in practice, this would query actual metrics
	return 5.0 // 5% error rate
}

func (am *AlertManager) getCurrentResponseTime(service string) float64 {
	// Simplified - in practice, this would query actual metrics
	return 150.0 // 150ms average response time
}

func (am *AlertManager) getCurrentMemoryUsage() float64 {
	// Simplified - in practice, this would query actual metrics
	return 75.0 // 75% memory usage
}

func (am *AlertManager) getCurrentCPUUsage() float64 {
	// Simplified - in practice, this would query actual metrics
	return 60.0 // 60% CPU usage
}

// Predefined alert rules
var DefaultAlertRules = []AlertRule{
	{
		Name:        "HighErrorRate",
		Query:       "error_rate",
		Threshold:   10.0, // 10% error rate
		Operator:    "gt",
		Severity:    SeverityWarning,
		Service:     "api",
		Description: "Error rate is above 10%",
		For:         5 * time.Minute,
		Labels: map[string]string{
			"team": "backend",
		},
		Annotations: map[string]string{
			"summary":     "High error rate detected",
			"description": "The error rate for the API service is above 10% for the last 5 minutes",
		},
	},
	{
		Name:        "SlowResponseTime",
		Query:       "response_time",
		Threshold:   1000.0, // 1000ms
		Operator:    "gt",
		Severity:    SeverityWarning,
		Service:     "api",
		Description: "Response time is above 1000ms",
		For:         2 * time.Minute,
		Labels: map[string]string{
			"team": "backend",
		},
		Annotations: map[string]string{
			"summary":     "Slow response time detected",
			"description": "The average response time is above 1000ms for the last 2 minutes",
		},
	},
	{
		Name:        "HighMemoryUsage",
		Query:       "memory_usage",
		Threshold:   90.0, // 90%
		Operator:    "gt",
		Severity:    SeverityCritical,
		Service:     "system",
		Description: "Memory usage is above 90%",
		For:         1 * time.Minute,
		Labels: map[string]string{
			"team": "platform",
		},
		Annotations: map[string]string{
			"summary":     "High memory usage detected",
			"description": "System memory usage is above 90% for the last minute",
		},
	},
	{
		Name:        "HighCPUUsage",
		Query:       "cpu_usage",
		Threshold:   80.0, // 80%
		Operator:    "gt",
		Severity:    SeverityError,
		Service:     "system",
		Description: "CPU usage is above 80%",
		For:         3 * time.Minute,
		Labels: map[string]string{
			"team": "platform",
		},
		Annotations: map[string]string{
			"summary":     "High CPU usage detected",
			"description": "System CPU usage is above 80% for the last 3 minutes",
		},
	},
}

// Global alert manager instance
var globalAlertManager *AlertManager

// InitGlobalAlertManager initializes the global alert manager
func InitGlobalAlertManager(logger *Logger, checkInterval time.Duration) {
	globalAlertManager = NewAlertManager(logger, checkInterval)

	// Add default alert rules
	for _, rule := range DefaultAlertRules {
		globalAlertManager.AddRule(rule)
	}
}

// GetGlobalAlertManager returns the global alert manager
func GetGlobalAlertManager() *AlertManager {
	return globalAlertManager
}

// StartGlobalAlerting starts the global alert manager
func StartGlobalAlerting(ctx context.Context) {
	if globalAlertManager != nil {
		go globalAlertManager.Start(ctx)
	}
}
