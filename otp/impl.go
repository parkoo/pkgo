package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// otpImpl is the default OTP implementation backed by any Store.
type otpImpl struct {
	cfg   Config
	store Store
}

// New creates an OTP instance backed by the given Store.
func New(cfg Config, store Store) OTP {
	cfg.applyDefaults()
	return &otpImpl{cfg: cfg, store: store}
}

// Generate creates and stores a verification code with rate-limit enforcement.
//
// Flow: check lock → check cooldown → check daily limit → check IP limit → generate code → store.
//
// Keys set:
//   - otp:lock:{bizType}:{target}   — lock flag, TTL=LockSec. Blocks both Generate and Verify when present.
//   - otp:cd:{bizType}:{target}     — cooldown marker, TTL=CooldownSec. Prevents rapid re-send.
//   - otp:daily:{bizType}:{target}  — daily send counter, expires at end of day.
//   - otp:ip:{ip}                   — per-IP daily counter, expires at end of day.
//   - otp:code:{bizType}:{target}   — the actual code, TTL=ExpireSec. Consumed on successful Verify.
func (r *otpImpl) Generate(ctx context.Context, bizType, target, ip string) (string, error) {
	// check lock (target may be locked due to too many verify errors)
	lockKey := r.key("lock", bizType, target)
	locked, err := r.store.Exists(ctx, lockKey)
	if err != nil {
		return "", fmt.Errorf("otp check lock failed, err: %v", err)
	}
	if locked {
		return "", ErrLocked
	}

	// check cooldown
	cdKey := r.key("cd", bizType, target)
	cdExists, err := r.store.Exists(ctx, cdKey)
	if err != nil {
		return "", fmt.Errorf("otp check cooldown failed, err: %v", err)
	}
	if cdExists {
		return "", ErrCooldown
	}

	// check daily limit for target
	dailyKey := r.key("daily", bizType, target)
	count, err := r.store.Incr(ctx, dailyKey)
	if err != nil {
		return "", fmt.Errorf("otp incr daily counter failed, err: %v", err)
	}
	if count == 1 {
		// first send today, set expiration to end of day
		_ = r.store.Expire(ctx, dailyKey, untilEndOfDay())
	}
	if count > int64(r.cfg.DailyLimit) {
		return "", ErrDailyLimit
	}

	// check IP daily limit (if ip provided)
	if ip != "" {
		ipKey := r.key("ip", "", ip)
		ipCount, err := r.store.Incr(ctx, ipKey)
		if err != nil {
			return "", fmt.Errorf("otp incr ip counter failed, err: %v", err)
		}
		if ipCount == 1 {
			_ = r.store.Expire(ctx, ipKey, untilEndOfDay())
		}
		if ipCount > int64(r.cfg.IPDailyLimit) {
			return "", ErrIPDailyLimit
		}
	}

	// generate code (use fixed debug code if configured)
	var code string
	if r.cfg.DebugCode != "" {
		code = r.cfg.DebugCode
	} else {
		var err2 error
		code, err2 = generateCode(r.cfg.CodeLength)
		if err2 != nil {
			return "", fmt.Errorf("otp generate code failed, err: %v", err2)
		}
	}

	// store code with TTL
	codeKey := r.key("code", bizType, target)
	if err := r.store.Set(ctx, codeKey, code, time.Duration(r.cfg.ExpireSec)*time.Second); err != nil {
		return "", fmt.Errorf("otp store code failed, err: %v", err)
	}

	// set cooldown marker
	if err := r.store.Set(ctx, cdKey, "1", time.Duration(r.cfg.CooldownSec)*time.Second); err != nil {
		return "", fmt.Errorf("otp set cooldown failed, err: %v", err)
	}

	return code, nil
}

// Verify checks the provided code and enforces error-count locking.
//
// Flow: check lock → get stored code → compare → on mismatch incr errors (may lock) → on match delete code.
//
// Keys used:
//   - otp:code:{bizType}:{target}  — DELETE on success. One-time use, prevents replay.
//   - otp:err:{bizType}:{target}   — DELETE on success. Resets error count so next code starts fresh.
//     On mismatch: INCR, when reaches MaxErrors → sets lock key and cleans up code+err.
//   - otp:lock:{bizType}:{target}  — NOT deleted. If locked, request is rejected early; wait for TTL expiry.
//   - otp:cd:{bizType}:{target}    — NOT deleted. Cooldown is a send-frequency limit, unrelated to verify result.
//   - otp:daily:{bizType}:{target} — NOT deleted. Daily quota is an abuse limit, not reset by a single success.
//   - otp:ip:{ip}                  — NOT deleted. Same reason as daily quota.
func (r *otpImpl) Verify(ctx context.Context, bizType, target, code string) error {
	// check lock
	lockKey := r.key("lock", bizType, target)
	locked, err := r.store.Exists(ctx, lockKey)
	if err != nil {
		return fmt.Errorf("otp check lock failed, err: %v", err)
	}
	if locked {
		return ErrLocked
	}

	// get stored code
	codeKey := r.key("code", bizType, target)
	stored, err := r.store.Get(ctx, codeKey)
	if err != nil {
		return fmt.Errorf("otp get code failed, err: %v", err)
	}
	if stored == "" {
		return ErrInvalidCode
	}

	// compare
	if stored != code {
		// increment error counter
		errKey := r.key("err", bizType, target)
		errCount, _ := r.store.Incr(ctx, errKey)
		if errCount == 1 {
			_ = r.store.Expire(ctx, errKey, time.Duration(r.cfg.LockSec)*time.Second)
		}
		if errCount >= int64(r.cfg.MaxErrors) {
			// lock target
			_ = r.store.Set(ctx, lockKey, "1", time.Duration(r.cfg.LockSec)*time.Second)
			// clean up code and error counter
			_ = r.store.Del(ctx, codeKey)
			_ = r.store.Del(ctx, errKey)
			return ErrLocked
		}
		return ErrInvalidCode
	}

	// success: delete code (one-time use) and error counter
	_ = r.store.Del(ctx, codeKey)
	errKey := r.key("err", bizType, target)
	_ = r.store.Del(ctx, errKey)

	return nil
}

// key builds a namespaced Redis key.
// format: otp:{category}:{bizType}:{identifier}
func (r *otpImpl) key(category, bizType, identifier string) string {
	parts := []string{"otp", category}
	if bizType != "" {
		parts = append(parts, bizType)
	}
	parts = append(parts, identifier)
	return strings.Join(parts, ":")
}

// generateCode generates a cryptographically random numeric code of given length.
func generateCode(length int) (string, error) {
	max := new(big.Int)
	max.SetString(strings.Repeat("9", length), 10)
	max.Add(max, big.NewInt(1)) // [0, 10^length)

	min := new(big.Int)
	min.SetString("1"+strings.Repeat("0", length-1), 10) // 10^(length-1)

	// range: [min, max) → guarantees exact digit count
	rangeSize := new(big.Int).Sub(max, min)
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return "", err
	}
	n.Add(n, min)

	return n.String(), nil
}

// untilEndOfDay returns the duration until the end of today (23:59:59).
func untilEndOfDay() time.Duration {
	now := time.Now()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	d := endOfDay.Sub(now)
	if d <= 0 {
		d = 24 * time.Hour
	}
	return d
}
