package token

import (
	"testing"
	"time"
)

func newTestManager(t *testing.T) Manager {
	t.Helper()
	m, err := New(Config{
		AccessSecret:  "test-access-secret-32bytes!!!!!",
		RefreshSecret: "test-refresh-secret-32bytes!!!!",
		AccessExpire:  5 * time.Second,
		RefreshExpire: 30 * time.Second,
		Issuer:        "test",
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return m
}

func TestGeneratePairAndVerify(t *testing.T) {
	m := newTestManager(t)

	pair, err := m.GeneratePair(Claims{
		UserID: "user_123",
		Extra:  map[string]string{"role": "admin"},
	})
	if err != nil {
		t.Fatalf("GeneratePair failed: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens should not be empty")
	}
	if pair.RefreshID == "" {
		t.Fatal("RefreshID (jti) should not be empty")
	}

	// verify access token
	claims, err := m.VerifyAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccess failed: %v", err)
	}
	if claims.UserID != "user_123" {
		t.Fatalf("expected UserID=user_123, got: %s", claims.UserID)
	}
	if claims.Extra["role"] != "admin" {
		t.Fatalf("expected Extra[role]=admin, got: %s", claims.Extra["role"])
	}

	// verify refresh token
	rClaims, jti, err := m.VerifyRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("VerifyRefresh failed: %v", err)
	}
	if rClaims.UserID != "user_123" {
		t.Fatalf("expected UserID=user_123, got: %s", rClaims.UserID)
	}
	if jti != pair.RefreshID {
		t.Fatalf("jti mismatch: got %s, want %s", jti, pair.RefreshID)
	}
}

func TestAccessTokenExpired(t *testing.T) {
	m, err := New(Config{
		AccessSecret:  "test-access-secret-32bytes!!!!!",
		RefreshSecret: "test-refresh-secret-32bytes!!!!",
		AccessExpire:  1 * time.Second,
		RefreshExpire: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	pair, err := m.GeneratePair(Claims{UserID: "user_456"})
	if err != nil {
		t.Fatalf("GeneratePair failed: %v", err)
	}

	// wait for access token to expire
	time.Sleep(1200 * time.Millisecond)

	_, err = m.VerifyAccess(pair.AccessToken)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got: %v", err)
	}

	// refresh token should still be valid
	_, _, err = m.VerifyRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken should still be valid: %v", err)
	}
}

func TestCrossVerifyRejected(t *testing.T) {
	m := newTestManager(t)

	pair, err := m.GeneratePair(Claims{UserID: "user_789"})
	if err != nil {
		t.Fatalf("GeneratePair failed: %v", err)
	}

	// access token should not pass refresh verification (different secret + type)
	_, _, err = m.VerifyRefresh(pair.AccessToken)
	if err == nil {
		t.Fatal("access token should not pass VerifyRefresh")
	}

	// refresh token should not pass access verification
	_, err = m.VerifyAccess(pair.RefreshToken)
	if err == nil {
		t.Fatal("refresh token should not pass VerifyAccess")
	}
}

func TestRefresh(t *testing.T) {
	m := newTestManager(t)

	pair1, err := m.GeneratePair(Claims{UserID: "user_abc"})
	if err != nil {
		t.Fatalf("GeneratePair failed: %v", err)
	}

	// refresh to get new pair
	pair2, err := m.Refresh(pair1.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// new pair should have different tokens and different jti
	if pair2.AccessToken == pair1.AccessToken {
		t.Fatal("new access token should differ")
	}
	if pair2.RefreshToken == pair1.RefreshToken {
		t.Fatal("new refresh token should differ")
	}
	if pair2.RefreshID == pair1.RefreshID {
		t.Fatal("new RefreshID (jti) should differ")
	}

	// new access token should be valid
	claims, err := m.VerifyAccess(pair2.AccessToken)
	if err != nil {
		t.Fatalf("new access token VerifyAccess failed: %v", err)
	}
	if claims.UserID != "user_abc" {
		t.Fatalf("expected user_abc, got: %s", claims.UserID)
	}
}

func TestEmptySecret(t *testing.T) {
	_, err := New(Config{
		AccessSecret:  "",
		RefreshSecret: "something",
	})
	if err != ErrSecretEmpty {
		t.Fatalf("expected ErrSecretEmpty, got: %v", err)
	}

	_, err = New(Config{
		AccessSecret:  "something",
		RefreshSecret: "",
	})
	if err != ErrSecretEmpty {
		t.Fatalf("expected ErrSecretEmpty, got: %v", err)
	}
}

func TestInvalidToken(t *testing.T) {
	m := newTestManager(t)

	_, err := m.VerifyAccess("not.a.valid.token")
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got: %v", err)
	}

	_, _, err = m.VerifyRefresh("garbage")
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got: %v", err)
	}
}
