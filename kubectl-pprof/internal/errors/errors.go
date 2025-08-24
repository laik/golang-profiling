package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeKubernetes    ErrorType = "kubernetes"
	ErrorTypeProfiler      ErrorType = "profiler"
	ErrorTypeIO            ErrorType = "io"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypePermission    ErrorType = "permission"
	ErrorTypeNetwork       ErrorType = "network"
)

// ProfileError represents a structured error with context
type ProfileError struct {
	Type        ErrorType
	Message     string
	Cause       error
	Suggestions []string
	Retryable   bool
}

// Error implements the error interface
func (e *ProfileError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *ProfileError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *ProfileError) IsRetryable() bool {
	return e.Retryable
}

// GetSuggestions returns user-friendly suggestions for fixing the error
func (e *ProfileError) GetSuggestions() []string {
	return e.Suggestions
}

// FormatUserMessage returns a user-friendly error message with suggestions
func (e *ProfileError) FormatUserMessage() string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("❌ %s\n", e.Message))
	
	if len(e.Suggestions) > 0 {
		builder.WriteString("\n💡 Suggestions:\n")
		for i, suggestion := range e.Suggestions {
			builder.WriteString(fmt.Sprintf("   %d. %s\n", i+1, suggestion))
		}
	}
	
	return builder.String()
}

// NewValidationError creates a new validation error
func NewValidationError(message string, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeValidation,
		Message:     message,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(message string, cause error, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeConfiguration,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// NewKubernetesError creates a new Kubernetes-related error
func NewKubernetesError(message string, cause error, retryable bool, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeKubernetes,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   retryable,
	}
}

// NewProfilerError creates a new profiler-related error
func NewProfilerError(message string, cause error, retryable bool, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeProfiler,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   retryable,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(message string, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeTimeout,
		Message:     message,
		Suggestions: suggestions,
		Retryable:   true,
	}
}

// NewPermissionError creates a new permission error
func NewPermissionError(message string, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypePermission,
		Message:     message,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// NewIOError creates a new I/O error
func NewIOError(message string, cause error, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeIO,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   true,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, cause error, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        ErrorTypeNetwork,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   true,
	}
}

// WrapError wraps an existing error with additional context
func WrapError(err error, errorType ErrorType, message string, suggestions ...string) *ProfileError {
	return &ProfileError{
		Type:        errorType,
		Message:     message,
		Cause:       err,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// IsProfileError checks if an error is a ProfileError
func IsProfileError(err error) bool {
	_, ok := err.(*ProfileError)
	return ok
}

// GetProfileError extracts ProfileError from an error chain
func GetProfileError(err error) *ProfileError {
	if profileErr, ok := err.(*ProfileError); ok {
		return profileErr
	}
	return nil
}