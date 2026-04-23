package model

import "time"

// RetryableError is implemented by provider-specific errors that can
// declare whether they are transient and suggest a retry-after delay.
type RetryableError interface {
	error
	IsRetryable() bool
	RetryAfter() time.Duration
}
