package otp

import (
	"context"
	"time"
)

// Store abstracts the underlying storage for OTP data.
// Implementations: Redis (production), Memory (dev/test).
type Store interface {
	// Set stores a key-value pair with expiration.
	Set(ctx context.Context, key, value string, exp time.Duration) error

	// Get retrieves the value for a key. Returns "" if not found.
	Get(ctx context.Context, key string) (string, error)

	// Del deletes a key.
	Del(ctx context.Context, key string) error

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Incr increments a counter and returns the new value.
	// If the key does not exist, it is set to 1.
	Incr(ctx context.Context, key string) (int64, error)

	// Expire sets expiration on an existing key.
	Expire(ctx context.Context, key string, exp time.Duration) error
}
