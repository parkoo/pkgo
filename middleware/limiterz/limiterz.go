// Package limiter provides HTTP rate-limiting middleware for go-zero,
// backed by an in-memory token bucket (ulule/limiter).
package limiterz

import (
	"context"
	"net/http"
	"strconv"

	"github.com/parkoo/pkgo/logger/logz"
	lulu_limiter "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"github.com/zeromicro/go-zero/rest"
)

// NewLimiter creates a rate-limiting middleware.
// The rate string follows ulule/limiter format: "<limit>-<period>",
// e.g. "100-M" (100 req/min), "5-S" (5 req/sec).
//
// Rate limiting is keyed by IP + Path + X-User-Id, so each user-path
// combination has an independent token bucket.
//
// On failure (store error), the middleware fails open (allows the request)
// and logs the error.
func NewLimiter(rate string) (rest.Middleware, error) {
	_limiter, err := buildLocalLimiter(rate, true)
	if err != nil {
		return nil, err
	}

	// keyGetter builds a unique rate-limit key per request.
	// Length-prefixed encoding prevents collision when field values contain delimiters.
	// Example: "11:192.168.1.1|8:/api/foo|6:user-1"
	keyGetter := func(r *http.Request) string {
		ip := _limiter.GetIPKey(r)
		path := r.URL.Path
		uid := r.Header.Get("X-User-Id")
		return strconv.Itoa(len(ip)) + ":" + ip + "|" +
			strconv.Itoa(len(path)) + ":" + path + "|" +
			strconv.Itoa(len(uid)) + ":" + uid
	}

	onLimitReached := stdlib.DefaultLimitReachedHandler

	// Package-level logger for middleware initialization errors.
	log := logz.WithCtx(context.Background())

	limiter := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := keyGetter(r)

			// Consume one token from the bucket.
			lctx, err := _limiter.Get(r.Context(), key)
			if err != nil {
				// Fail open: allow the request but log the error.
				log.Errorf("[pkgo] limiter get key failed, err: %v", err)
				next(w, r)
				return
			}

			// Set (not Add) to ensure a single authoritative value per header.
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(lctx.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(lctx.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(lctx.Reset, 10))

			if lctx.Reached {
				onLimitReached(w, r)
				return
			}

			next(w, r)
		}
	}

	return limiter, nil
}

// buildLocalLimiter constructs a limiter instance with an in-memory store.
// trustForwardHeader controls whether X-Forwarded-For / X-Real-IP is used for IP resolution.
func buildLocalLimiter(rateStr string, trustForwardHeader bool) (*lulu_limiter.Limiter, error) {
	rate, err := lulu_limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		return nil, err
	}

	store := memory.NewStore()
	_limiter := lulu_limiter.New(store, rate, lulu_limiter.WithTrustForwardHeader(trustForwardHeader))
	return _limiter, nil
}
