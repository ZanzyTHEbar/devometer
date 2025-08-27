package errors

import (
	"fmt"
	"testing"

	"github.com/ZanzyTHEbar/errbuilder-go"
)

// TestIntegrationErrBuilder tests the errbuilder integration
func TestIntegrationErrBuilder(t *testing.T) {
	// Test basic error creation
	validationErr := NewValidationError("test validation error", "field1", "value1")
	if validationErr == nil {
		t.Fatal("Expected validation error to be created")
	}

	// Test error message
	expectedMsg := "[VALIDATION_ERROR] test validation error"
	if validationErr.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, validationErr.Error())
	}

	// Test error category
	if validationErr.Category != CategoryValidation {
		t.Errorf("Expected category %v, got %v", CategoryValidation, validationErr.Category)
	}

	// Test HTTP status
	if validationErr.HTTPStatus != 400 {
		t.Errorf("Expected HTTP status 400, got %d", validationErr.HTTPStatus)
	}

	// Test network error
	networkErr := NewNetworkError("connection failed", fmt.Errorf("connection refused"))
	if networkErr == nil {
		t.Fatal("Expected network error to be created")
	}

	if networkErr.Category != CategoryNetwork {
		t.Errorf("Expected category %v, got %v", CategoryNetwork, networkErr.Category)
	}

	// Test error map functionality
	errMap := NewErrorMap()
	errMap.Set("field1", NewValidationError("field1 error"))
	errMap.Set("field2", NewValidationError("field2 error"))

	validationErrWithMap := NewValidationErrorWithMap(map[string]string{
		"field1": "field1 error",
		"field2": "field2 error",
	})

	if validationErrWithMap == nil {
		t.Fatal("Expected validation error with map to be created")
	}

	// Test errbuilder builder pattern
	builder := NewBuilder().
		WithCode(errbuilder.CodeInvalidArgument).
		WithMsg("Custom error message")

	customErr := NewAppError(builder, CategoryValidation, 400)
	if customErr == nil {
		t.Fatal("Expected custom error to be created")
	}

	if customErr.Msg != "Custom error message" {
		t.Errorf("Expected custom message, got %q", customErr.Msg)
	}

	// Test error conversion
	standardErr := fmt.Errorf("standard error")
	convertedErr := ToAppError(standardErr)
	if convertedErr == nil {
		t.Fatal("Expected error to be converted")
	}

	// Test retry logic
	if !IsRetryableError(networkErr) {
		t.Error("Expected network error to be retryable")
	}

	if IsRetryableError(validationErr) {
		t.Error("Expected validation error to not be retryable")
	}

	// Test delay calculation
	delay := GetRetryDelay(networkErr, 1)
	if delay <= 0 {
		t.Error("Expected positive retry delay")
	}

	fmt.Println("✅ All errbuilder integration tests passed!")
}
