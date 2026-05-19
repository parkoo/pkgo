package logz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/parkoo/pkgo/context/tag"
	gormlogger "gorm.io/gorm/logger"
)

func TestWithCtx(t *testing.T) {
	ctx := context.Background()
	ts := tag.NewTags()
	ts.Set("request-id", "test_req_id")
	ctx = tag.SetInContext(ctx, ts)

	// ensure the returned logger is not nil
	logD := WithCtx(ctx)
	if logD == nil {
		t.Fatal("WithCtx returned nil logger")
	}
	logD.Debug("test debug")
	logD.Debugf("test debugf, info: %v", "testDebugf")

	// Tags is a pointer; no need to call SetInContext again after Set
	ts.Set("user-id", "test_user_id")
	logI := WithCtx(ctx)
	if logI == nil {
		t.Fatal("WithCtx returned nil logger after adding tag")
	}
	logI.Info("test info")
	logI.Infof("test infof, info: %v", "testInfof")

	logE := WithCtx(ctx)
	if logE == nil {
		t.Fatal("WithCtx returned nil logger for error log")
	}
	logE.Error("test error")
	logE.Errorf("test errorf, error: %v", "testErrorf")
}

func TestWithCtx_NilContext(t *testing.T) {
	// ensure nil context does not panic
	logger := WithCtx(nil) //nolint:staticcheck // intentionally testing nil context safety
	if logger == nil {
		t.Fatal("WithCtx(nil) returned nil logger")
	}
	logger.Info("log with nil context")
}

func TestWithCtx_EmptyTags(t *testing.T) {
	// should work when no Tags exist in context
	ctx := context.Background()
	logger := WithCtx(ctx)
	if logger == nil {
		t.Fatal("WithCtx returned nil logger when no tags in context")
	}
	logger.Info("log with empty tags")
}

func TestTags_Concurrent(t *testing.T) {
	tags := tag.NewTags()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			tags.Set(fmt.Sprintf("key-%d", i), i)
		}()
		go func() {
			defer wg.Done()
			_ = tags.Values()
		}()
		go func() {
			defer wg.Done()
			_ = tags.Has(fmt.Sprintf("key-%d", i))
		}()
	}
	wg.Wait()

	// verify all keys were written successfully
	for i := range 100 {
		if !tags.Has(fmt.Sprintf("key-%d", i)) {
			t.Errorf("key-%d not found after concurrent writes", i)
		}
	}
}

func TestTags_Get(t *testing.T) {
	tags := tag.NewTags()
	tags.Set("foo", "bar")

	v, ok := tags.Get("foo")
	if !ok {
		t.Fatal("expected key 'foo' to exist")
	}
	if v != "bar" {
		t.Fatalf("expected value 'bar', got %v", v)
	}

	_, ok = tags.Get("nonexistent")
	if ok {
		t.Fatal("expected key 'nonexistent' to not exist")
	}
}

func TestHas_NilContext(t *testing.T) {
	// package-level Has should not panic on nil context
	if tag.Has(nil) { //nolint:staticcheck // intentionally testing nil context safety
		t.Fatal("Has(nil) should return false")
	}
}

// ---------- NewGormLogger tests ----------

// TestNewGormLogger_ReturnsNonNil verifies the constructor produces a valid interface.
func TestNewGormLogger_ReturnsNonNil(t *testing.T) {
	gl := NewGormLogger(gormlogger.Info, 200*time.Millisecond)
	if gl == nil {
		t.Fatal("expected non-nil gorm logger")
	}
}

// TestGormLogger_LogMode returns a new instance with updated level.
func TestGormLogger_LogMode(t *testing.T) {
	gl := NewGormLogger(gormlogger.Info, 200*time.Millisecond)

	newGL := gl.LogMode(gormlogger.Silent)
	if newGL == nil {
		t.Fatal("LogMode returned nil")
	}

	// Original should be unchanged (new instance returned).
	orig := gl.(*gormLogger)
	updated := newGL.(*gormLogger)
	if orig.level == updated.level {
		t.Error("LogMode should return a new instance with different level")
	}
	if updated.level != gormlogger.Silent {
		t.Errorf("expected level Silent(%d), got %d", gormlogger.Silent, updated.level)
	}
	// slowThreshold should be preserved.
	if updated.slowThreshold != 200*time.Millisecond {
		t.Errorf("expected slowThreshold preserved, got %v", updated.slowThreshold)
	}
}

// TestGormLogger_Info does not panic and respects level filtering.
func TestGormLogger_Info(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-info-test")

	// Level=Info should log.
	gl := NewGormLogger(gormlogger.Info, 200*time.Millisecond)
	gl.Info(ctx, "info message: %s", "hello")

	// Level=Warn (lower verbosity) should suppress Info messages — no panic.
	gl2 := NewGormLogger(gormlogger.Warn, 200*time.Millisecond)
	gl2.Info(ctx, "this should be suppressed")
}

// TestGormLogger_Warn does not panic and respects level filtering.
func TestGormLogger_Warn(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-warn-test")

	gl := NewGormLogger(gormlogger.Warn, 200*time.Millisecond)
	gl.Warn(ctx, "warn message: %s", "slow query")

	// Level=Error should suppress Warn.
	gl2 := NewGormLogger(gormlogger.Error, 200*time.Millisecond)
	gl2.Warn(ctx, "this should be suppressed")
}

// TestGormLogger_Error does not panic and respects level filtering.
func TestGormLogger_Error(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-error-test")

	gl := NewGormLogger(gormlogger.Error, 200*time.Millisecond)
	gl.Error(ctx, "error message: %v", errors.New("db connection lost"))

	// Level=Silent should suppress everything.
	gl2 := NewGormLogger(gormlogger.Silent, 200*time.Millisecond)
	gl2.Error(ctx, "this should be suppressed")
}

// TestGormLogger_Trace_WithError logs error trace when an error is provided.
func TestGormLogger_Trace_WithError(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-trace-err")
	gl := NewGormLogger(gormlogger.Info, 200*time.Millisecond)

	begin := time.Now().Add(-50 * time.Millisecond) // simulate 50ms elapsed
	fc := func() (string, int64) {
		return "SELECT * FROM user WHERE id = 1", 1
	}

	// Should not panic; logs as error trace.
	gl.Trace(ctx, begin, fc, errors.New("record not found"))
}

// TestGormLogger_Trace_SlowQuery logs slow trace when elapsed exceeds threshold.
func TestGormLogger_Trace_SlowQuery(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-trace-slow")
	gl := NewGormLogger(gormlogger.Warn, 100*time.Millisecond)

	begin := time.Now().Add(-500 * time.Millisecond) // simulate 500ms (exceeds 100ms threshold)
	fc := func() (string, int64) {
		return "SELECT * FROM order WHERE user_id = 42", 10
	}

	// Should not panic; logs as slow query.
	gl.Trace(ctx, begin, fc, nil)
}

// TestGormLogger_Trace_NormalQuery logs info trace for normal queries.
func TestGormLogger_Trace_NormalQuery(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-trace-normal")
	gl := NewGormLogger(gormlogger.Info, 200*time.Millisecond)

	begin := time.Now().Add(-5 * time.Millisecond) // simulate 5ms (fast)
	fc := func() (string, int64) {
		return "INSERT INTO user(name) VALUES('test')", 1
	}

	// Should not panic; logs as normal info trace.
	gl.Trace(ctx, begin, fc, nil)
}

// TestGormLogger_Trace_Silent suppresses all trace output.
func TestGormLogger_Trace_Silent(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-trace-silent")
	gl := NewGormLogger(gormlogger.Silent, 200*time.Millisecond)

	begin := time.Now().Add(-500 * time.Millisecond)
	fc := func() (string, int64) {
		return "DELETE FROM user WHERE id = 99", 1
	}

	// Silent level should skip all trace logic — no panic.
	gl.Trace(ctx, begin, fc, errors.New("some error"))
}

// TestGormLogger_Trace_ZeroThreshold disables slow-query detection when threshold is 0.
func TestGormLogger_Trace_ZeroThreshold(t *testing.T) {
	ctx := contextWithTags("trace-id", "gorm-trace-zero-threshold")
	gl := NewGormLogger(gormlogger.Info, 0) // threshold=0 disables slow detection

	begin := time.Now().Add(-10 * time.Second) // very slow, but threshold is 0
	fc := func() (string, int64) {
		return "SELECT 1", 0
	}

	// Should log as normal info trace (not slow), no panic.
	gl.Trace(ctx, begin, fc, nil)
}

// contextWithTags is a test helper that creates a context with pre-set tags.
func contextWithTags(key string, value interface{}) context.Context {
	ctx := context.Background()
	ts := tag.NewTags()
	ts.Set(key, value)
	return tag.SetInContext(ctx, ts)
}
