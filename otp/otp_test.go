package otp

import (
	"context"
	"testing"
	"time"
)

func TestGenerateAndVerify(t *testing.T) {
	store := NewMemoryStore()
	o := New(Config{
		CodeLength:  6,
		ExpireSec:   60,
		CooldownSec: 2,
		DailyLimit:  10,
		MaxErrors:   3,
		LockSec:     10,
	}, store)

	ctx := context.Background()

	// generate code
	code, err := o.Generate(ctx, "register", "target_hash_1", "127.0.0.1")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got: %s", code)
	}

	// verify with wrong code
	err = o.Verify(ctx, "register", "target_hash_1", "000000")
	if err != ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode, got: %v", err)
	}

	// verify with correct code
	err = o.Verify(ctx, "register", "target_hash_1", code)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	// verify again should fail (one-time use)
	err = o.Verify(ctx, "register", "target_hash_1", code)
	if err != ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode after consumption, got: %v", err)
	}
}

func TestCooldown(t *testing.T) {
	store := NewMemoryStore()
	o := New(Config{
		CooldownSec: 2,
		DailyLimit:  10,
	}, store)

	ctx := context.Background()

	_, err := o.Generate(ctx, "login", "target_hash_2", "")
	if err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}

	// immediate second request should hit cooldown
	_, err = o.Generate(ctx, "login", "target_hash_2", "")
	if err != ErrCooldown {
		t.Fatalf("expected ErrCooldown, got: %v", err)
	}

	// wait for cooldown to expire
	time.Sleep(2100 * time.Millisecond)

	_, err = o.Generate(ctx, "login", "target_hash_2", "")
	if err != nil {
		t.Fatalf("Generate after cooldown should succeed: %v", err)
	}
}

func TestDailyLimit(t *testing.T) {
	store := NewMemoryStore()
	o := New(Config{
		CooldownSec: 0,
		DailyLimit:  3,
	}, store)

	ctx := context.Background()

	// override cooldown by setting it to 0 (will default to 60)
	// use a fresh instance with 1-sec cooldown instead
	o = New(Config{
		CooldownSec: 1,
		DailyLimit:  3,
	}, store)

	for i := 0; i < 3; i++ {
		_, err := o.Generate(ctx, "test", "daily_target", "")
		if err != nil {
			t.Fatalf("Generate #%d failed: %v", i+1, err)
		}
		time.Sleep(1100 * time.Millisecond) // wait for cooldown
	}

	// 4th should hit daily limit
	_, err := o.Generate(ctx, "test", "daily_target", "")
	if err != ErrDailyLimit {
		t.Fatalf("expected ErrDailyLimit, got: %v", err)
	}
}

func TestErrorLocking(t *testing.T) {
	store := NewMemoryStore()
	o := New(Config{
		CooldownSec: 1,
		MaxErrors:   3,
		LockSec:     5,
	}, store)

	ctx := context.Background()

	code, err := o.Generate(ctx, "register", "lock_target", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	_ = code

	// 3 wrong attempts → should lock
	for i := 0; i < 3; i++ {
		err = o.Verify(ctx, "register", "lock_target", "wrong!")
		if i < 2 {
			if err != ErrInvalidCode {
				t.Fatalf("attempt %d: expected ErrInvalidCode, got: %v", i+1, err)
			}
		} else {
			if err != ErrLocked {
				t.Fatalf("attempt %d: expected ErrLocked, got: %v", i+1, err)
			}
		}
	}

	// subsequent verify should also return locked
	err = o.Verify(ctx, "register", "lock_target", code)
	if err != ErrLocked {
		t.Fatalf("expected ErrLocked after lockout, got: %v", err)
	}

	// generate should also return locked
	_, err = o.Generate(ctx, "register", "lock_target", "")
	if err != ErrLocked {
		t.Fatalf("expected ErrLocked on Generate, got: %v", err)
	}
}

func TestIPDailyLimit(t *testing.T) {
	store := NewMemoryStore()
	o := New(Config{
		CooldownSec:  1,
		DailyLimit:   100,
		IPDailyLimit: 2,
	}, store)

	ctx := context.Background()

	// use different targets but same IP
	_, err := o.Generate(ctx, "register", "ip_target_1", "1.2.3.4")
	if err != nil {
		t.Fatalf("Generate 1 failed: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)

	_, err = o.Generate(ctx, "register", "ip_target_2", "1.2.3.4")
	if err != nil {
		t.Fatalf("Generate 2 failed: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)

	// 3rd from same IP should fail
	_, err = o.Generate(ctx, "register", "ip_target_3", "1.2.3.4")
	if err != ErrIPDailyLimit {
		t.Fatalf("expected ErrIPDailyLimit, got: %v", err)
	}
}

func TestGenerateCodeLength(t *testing.T) {
	for _, length := range []int{4, 6, 8} {
		code, err := generateCode(length)
		if err != nil {
			t.Fatalf("generateCode(%d) failed: %v", length, err)
		}
		if len(code) != length {
			t.Fatalf("expected %d digits, got %q (len=%d)", length, code, len(code))
		}
	}
}
