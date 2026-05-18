package limiterz

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestNewLimiter_InvalidRate verifies that an invalid rate string returns an error.
func TestNewLimiter_InvalidRate(t *testing.T) {
	_, err := NewLimiter("invalid-rate")
	if err == nil {
		t.Fatal("expected error for invalid rate format, got nil")
	}
}

// TestNewLimiter_ValidRate verifies that a valid rate string produces a usable middleware.
func TestNewLimiter_ValidRate(t *testing.T) {
	mw, err := NewLimiter("5-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}
}

// TestLimiter_AllowsRequestsWithinLimit ensures requests within the quota
// pass through and receive correct rate-limit response headers.
func TestLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	mw, err := NewLimiter("10-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-User-Id", "user-123")

	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Verify rate-limit headers.
	if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "10" {
		t.Errorf("expected X-RateLimit-Limit=10, got %q", limit)
	}
	if remaining := rec.Header().Get("X-RateLimit-Remaining"); remaining != "9" {
		t.Errorf("expected X-RateLimit-Remaining=9, got %q", remaining)
	}
	if reset := rec.Header().Get("X-RateLimit-Reset"); reset == "" {
		t.Error("expected X-RateLimit-Reset to be set")
	}
}

// TestLimiter_BlocksRequestsOverLimit verifies that requests exceeding the
// quota receive 429 and the next handler is not invoked.
func TestLimiter_BlocksRequestsOverLimit(t *testing.T) {
	mw, err := NewLimiter("3-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nextCalled := 0
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})

	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("X-User-Id", "user-456")
		rec := httptest.NewRecorder()
		handler(rec, req)

		if i < 3 {
			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
			}
		} else {
			// Exceeds quota → 429
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d: expected 429, got %d", i, rec.Code)
			}
		}
	}

	if nextCalled != 3 {
		t.Errorf("expected next handler called 3 times, got %d", nextCalled)
	}
}

// TestLimiter_HeaderUsesSet_NotAdd ensures the middleware uses Set (overwrite)
// instead of Add (append), so upstream headers are replaced, not duplicated.
func TestLimiter_HeaderUsesSet_NotAdd(t *testing.T) {
	mw, err := NewLimiter("10-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	// Simulate upstream middleware having already set these headers.
	rec.Header().Set("X-RateLimit-Limit", "9999")
	rec.Header().Set("X-RateLimit-Remaining", "9999")

	handler(rec, req)

	// Expect exactly one value per header (Set overwrites).
	if vals := rec.Header().Values("X-RateLimit-Limit"); len(vals) != 1 {
		t.Errorf("expected 1 X-RateLimit-Limit value, got %d: %v", len(vals), vals)
	}
	if vals := rec.Header().Values("X-RateLimit-Remaining"); len(vals) != 1 {
		t.Errorf("expected 1 X-RateLimit-Remaining value, got %d: %v", len(vals), vals)
	}
}

// TestLimiter_DifferentKeysHaveIndependentLimits confirms that different users
// are rate-limited independently (separate buckets).
func TestLimiter_DifferentKeysHaveIndependentLimits(t *testing.T) {
	mw, err := NewLimiter("2-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// User A exhausts its 2-request quota.
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("X-User-Id", "user-A")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("user-A request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// User B should still have its full quota (independent bucket).
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("X-User-Id", "user-B")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("user-B request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// User A's next request is rejected.
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-User-Id", "user-A")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("user-A 3rd request: expected 429, got %d", rec.Code)
	}
}

// TestLimiter_DifferentPathsHaveIndependentLimits confirms that the same user
// hitting different paths gets separate rate-limit buckets.
func TestLimiter_DifferentPathsHaveIndependentLimits(t *testing.T) {
	mw, err := NewLimiter("2-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust quota on /api/path1.
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/path1", nil)
		req.Header.Set("X-User-Id", "user-X")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path1 request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// /api/path2 has its own bucket — should still pass.
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/api/path2", nil)
		req.Header.Set("X-User-Id", "user-X")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path2 request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

// TestLimiter_RemainingDecreases verifies the X-RateLimit-Remaining header
// decrements correctly with each request.
func TestLimiter_RemainingDecreases(t *testing.T) {
	mw, err := NewLimiter("5-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/api/counter", nil)
		req.Header.Set("X-User-Id", "counter-user")
		rec := httptest.NewRecorder()
		handler(rec, req)

		remaining, _ := strconv.ParseInt(rec.Header().Get("X-RateLimit-Remaining"), 10, 64)
		expected := int64(4 - i) // 4, 3, 2, 1, 0
		if remaining != expected {
			t.Errorf("request %d: expected remaining=%d, got %d", i, expected, remaining)
		}
	}
}

// TestLimiter_KeyCollisionAvoidance proves that the length-prefixed key format
// prevents collisions when field values contain the delimiter character.
//
// Without length-prefixing, these two requests would produce identical keys:
//   - Path="/a", UserID="b|c" → "IP|/a|b|c"
//   - Path="/a|b", UserID="c" → "IP|/a|b|c"   ← collision!
//
// With length-prefixing they differ:
//   - "...2:/a|3:b|c"
//   - "...4:/a|b|1:c"
func TestLimiter_KeyCollisionAvoidance(t *testing.T) {
	mw, err := NewLimiter("1-S")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request 1: Path="/a", UserID="b|c"
	req1 := httptest.NewRequest(http.MethodGet, "/a", nil)
	req1.Header.Set("X-User-Id", "b|c")
	rec1 := httptest.NewRecorder()
	handler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", rec1.Code)
	}

	// Request 2: Path="/a|b", UserID="c" — must be a different bucket.
	req2 := httptest.NewRequest(http.MethodGet, "/a|b", nil)
	req2.Header.Set("X-User-Id", "c")
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("request 2: expected 200 (different key), got %d", rec2.Code)
	}
}

// TestBuildLocalLimiter_InvalidRate verifies buildLocalLimiter rejects malformed rate strings.
func TestBuildLocalLimiter_InvalidRate(t *testing.T) {
	_, err := buildLocalLimiter("not-a-rate", true)
	if err == nil {
		t.Fatal("expected error for invalid rate string")
	}
}

// TestBuildLocalLimiter_ValidRate verifies buildLocalLimiter returns a usable limiter.
func TestBuildLocalLimiter_ValidRate(t *testing.T) {
	l, err := buildLocalLimiter("100-M", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil limiter")
	}
}
