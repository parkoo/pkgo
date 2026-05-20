package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Predefined errors for token operations.
var (
	ErrTokenExpired = errors.New("token: expired")
	ErrTokenInvalid = errors.New("token: invalid or malformed")
	ErrSecretEmpty  = errors.New("token: signing secret is empty")
)

const (
	claimUserID    = "uid"
	claimExtra     = "ext"
	claimTokenType = "typ"
	tokenAccess    = "access"
	tokenRefresh   = "refresh"
)

// jwtManager is the HS256-based Manager implementation.
type jwtManager struct {
	cfg Config
}

// New creates a Manager instance.
//
// Both AccessSecret and RefreshSecret must be non-empty.
// They should be different from each other for security.
func New(cfg Config) (Manager, error) {
	cfg.applyDefaults()
	if cfg.AccessSecret == "" || cfg.RefreshSecret == "" {
		return nil, ErrSecretEmpty
	}
	return &jwtManager{cfg: cfg}, nil
}

// GeneratePair creates access + refresh tokens.
//
// Access Token claims: uid, ext, typ="access", iss, iat, exp (no jti, short-lived).
// Refresh Token claims: uid, typ="refresh", iss, iat, exp, jti (for rotation detection).
func (m *jwtManager) GeneratePair(claims Claims) (*TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(m.cfg.AccessExpire)
	refreshExp := now.Add(m.cfg.RefreshExpire)
	refreshID := uuid.New().String()

	// build access token
	accessClaims := jwt.MapClaims{
		claimUserID:    claims.UserID,
		claimTokenType: tokenAccess,
		"iat":          now.Unix(),
		"exp":          accessExp.Unix(),
		"jti":          uuid.New().String(),
	}
	if m.cfg.Issuer != "" {
		accessClaims["iss"] = m.cfg.Issuer
	}
	if len(claims.Extra) > 0 {
		accessClaims[claimExtra] = claims.Extra
	}

	accessToken, err := m.sign(accessClaims, m.cfg.AccessSecret)
	if err != nil {
		return nil, fmt.Errorf("token sign access failed, err: %v", err)
	}

	// build refresh token (minimal claims + jti)
	refreshClaims := jwt.MapClaims{
		claimUserID:    claims.UserID,
		claimTokenType: tokenRefresh,
		"iat":          now.Unix(),
		"exp":          refreshExp.Unix(),
		"jti":          refreshID,
	}
	if m.cfg.Issuer != "" {
		refreshClaims["iss"] = m.cfg.Issuer
	}

	refreshToken, err := m.sign(refreshClaims, m.cfg.RefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("token sign refresh failed, err: %v", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccessExp:    accessExp,
		RefreshExp:   refreshExp,
		RefreshID:    refreshID,
	}, nil
}

// VerifyAccess validates an access token and extracts Claims.
func (m *jwtManager) VerifyAccess(tokenStr string) (*Claims, error) {
	mapClaims, err := m.parse(tokenStr, m.cfg.AccessSecret)
	if err != nil {
		return nil, err
	}

	// ensure token type is access
	if getStr(mapClaims, claimTokenType) != tokenAccess {
		return nil, ErrTokenInvalid
	}

	return extractClaims(mapClaims), nil
}

// VerifyRefresh validates a refresh token and returns Claims + jti.
// Caller must check jti against Redis to detect rotation reuse.
func (m *jwtManager) VerifyRefresh(tokenStr string) (*Claims, string, error) {
	mapClaims, err := m.parse(tokenStr, m.cfg.RefreshSecret)
	if err != nil {
		return nil, "", err
	}

	// ensure token type is refresh
	if getStr(mapClaims, claimTokenType) != tokenRefresh {
		return nil, "", ErrTokenInvalid
	}

	jti := getStr(mapClaims, "jti")
	if jti == "" {
		return nil, "", ErrTokenInvalid
	}

	return extractClaims(mapClaims), jti, nil
}

// Refresh verifies the refresh token and generates a new token pair.
// Note: caller should validate jti against Redis BEFORE calling this.
func (m *jwtManager) Refresh(refreshToken string) (*TokenPair, error) {
	claims, _, err := m.VerifyRefresh(refreshToken)
	if err != nil {
		return nil, err
	}
	return m.GeneratePair(*claims)
}

// sign creates a signed JWT string using HS256.
func (m *jwtManager) sign(claims jwt.MapClaims, secret string) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

// parse validates and parses a JWT token string.
func (m *jwtManager) parse(tokenStr, secret string) (jwt.MapClaims, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// extractClaims converts jwt.MapClaims to our Claims struct.
func extractClaims(mc jwt.MapClaims) *Claims {
	c := &Claims{
		UserID: getStr(mc, claimUserID),
	}
	// extract extra map
	if ext, ok := mc[claimExtra]; ok {
		if extMap, ok := ext.(map[string]interface{}); ok {
			c.Extra = make(map[string]string, len(extMap))
			for k, v := range extMap {
				if s, ok := v.(string); ok {
					c.Extra[k] = s
				}
			}
		}
	}
	return c
}

// getStr safely extracts a string value from MapClaims.
func getStr(mc jwt.MapClaims, key string) string {
	if v, ok := mc[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
