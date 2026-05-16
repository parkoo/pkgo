package logz

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/parkoo/pkgo/context/tag"
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
