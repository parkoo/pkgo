package token

import "time"

// Config holds JWT token configuration.
type Config struct {
	AccessSecret  string        // HMAC signing key for access token
	RefreshSecret string        // HMAC signing key for refresh token (should differ from access)
	AccessExpire  time.Duration // access token lifetime, default 15min
	RefreshExpire time.Duration // refresh token lifetime, default 7 days
	Issuer        string        // optional "iss" claim
}

// DefaultConfig provides sensible defaults for production use.
var DefaultConfig = Config{
	AccessExpire:  15 * time.Minute,
	RefreshExpire: 7 * 24 * time.Hour,
}

// applyDefaults fills zero-value fields with sensible defaults.
func (c *Config) applyDefaults() {
	if c.AccessExpire <= 0 {
		c.AccessExpire = 15 * time.Minute
	}
	if c.RefreshExpire <= 0 {
		c.RefreshExpire = 7 * 24 * time.Hour
	}
}

// Claims represents the custom payload embedded in tokens.
type Claims struct {
	UserID string            // primary user identifier
	Extra  map[string]string // extensible fields (role, device_id, etc.)
}

// TokenPair holds a generated access/refresh token pair.
type TokenPair struct {
	AccessToken  string    // short-lived token for API requests
	RefreshToken string    // long-lived token for obtaining new access tokens
	AccessExp    time.Time // access token expiration time
	RefreshExp   time.Time // refresh token expiration time
	RefreshID    string    // unique ID (jti) of the refresh token, store in Redis for rotation check
}

// Manager manages the lifecycle of JWT token pairs.
type Manager interface {
	// GeneratePair creates both access and refresh tokens.
	GeneratePair(claims Claims) (*TokenPair, error)

	// VerifyAccess validates an access token and returns the embedded claims.
	VerifyAccess(tokenStr string) (*Claims, error)

	// VerifyRefresh validates a refresh token and returns claims + token ID (jti).
	// The caller should check the returned jti against Redis to detect rotation reuse.
	VerifyRefresh(tokenStr string) (claims *Claims, jti string, err error)

	// Refresh is a convenience method: verify refresh token → generate new pair.
	// The caller is still responsible for Redis rotation checks before calling this.
	Refresh(refreshToken string) (*TokenPair, error)
}
