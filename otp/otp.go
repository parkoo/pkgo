package otp

import (
	"context"
	"errors"
)

// Predefined errors for OTP operations.
var (
	ErrCooldown     = errors.New("otp: send too frequently, please wait")
	ErrDailyLimit   = errors.New("otp: daily send limit exceeded")
	ErrIPDailyLimit = errors.New("otp: ip daily send limit exceeded")
	ErrLocked       = errors.New("otp: too many failed attempts, temporarily locked")
	ErrInvalidCode  = errors.New("otp: invalid or expired code")
)

// Config holds OTP behavior configuration.
type Config struct {
	CodeLength   int    // code digit count, default 6
	ExpireSec    int    // code expiration in seconds, default 300 (5min)
	CooldownSec  int    // min interval between sends to same target, default 60
	DailyLimit   int    // max sends per target per day, default 10
	IPDailyLimit int    // max sends per IP per day, default 20
	MaxErrors    int    // max verify errors before lock, default 5
	LockSec      int    // lock duration after max errors in seconds, default 900 (15min)
	DebugCode    string // if non-empty, Generate always returns this fixed code (for dev/test only)
}

// DefaultConfig provides sensible defaults for production use.
var DefaultConfig = Config{
	CodeLength:   6,
	ExpireSec:    300,
	CooldownSec:  60,
	DailyLimit:   10,
	IPDailyLimit: 20,
	MaxErrors:    5,
	LockSec:      900,
}

// applyDefaults fills zero-value fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.CodeLength <= 0 {
		c.CodeLength = 6
	}
	if c.ExpireSec <= 0 {
		c.ExpireSec = 300
	}
	if c.CooldownSec <= 0 {
		c.CooldownSec = 60
	}
	if c.DailyLimit <= 0 {
		c.DailyLimit = 10
	}
	if c.IPDailyLimit <= 0 {
		c.IPDailyLimit = 20
	}
	if c.MaxErrors <= 0 {
		c.MaxErrors = 5
	}
	if c.LockSec <= 0 {
		c.LockSec = 900
	}
}

// OTP manages the lifecycle of one-time verification codes.
type OTP interface {
	// Generate creates a code, enforces rate limits, and stores it.
	// bizType: business scenario, e.g. "register", "login", "reset_pwd"
	// target: hashed identifier (phone hash or email hash)
	// ip: client IP address for IP-level rate limiting (optional, pass "" to skip)
	Generate(ctx context.Context, bizType, target, ip string) (code string, err error)

	// Verify checks the code against stored value.
	// Returns nil on success; ErrInvalidCode or ErrLocked on failure.
	Verify(ctx context.Context, bizType, target, code string) error
}
