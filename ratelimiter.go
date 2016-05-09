package ratelimiter

import (
	"fmt"
	"net/http"
	"time"
)

type BucketStore interface {
	Take(key string) (taken bool, remaining int, resetUnixTime int64, err error)
}

type KeyFn func(r *http.Request) string

func Middleware(keyFn KeyFn, rate time.Duration, burst int) func(http.Handler) http.Handler {
	if burst < 1 {
		burst = 1
	}

	rateLimiter := rateLimiter{
		store:       MemoryStore(rate, burst),
		rate:        rate,
		keyFn:       keyFn,
		burst:       burst,
		rateHeader:  fmt.Sprintf("%d req/min", time.Minute/rate),
		resetHeader: fmt.Sprintf("%d", time.Now().Unix()),
	}

	fn := func(h http.Handler) http.Handler {
		rateLimiter.next = h
		return &rateLimiter
	}
	return fn
}

type rateLimiter struct {
	next        http.Handler
	store       BucketStore
	keyFn       KeyFn
	rate        time.Duration
	burst       int
	rateHeader  string
	resetHeader string
}

// ServeHTTPC implements http.Handler interface.
func (l *rateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := l.keyFn(r)
	ok, remaining, reset, err := l.store.Take(key)
	if err != nil {
		// TODO: Fallback stores?
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	if !ok {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	w.Header().Add("X-RateLimit-Key", key)
	w.Header().Add("X-RateLimit-Rate", l.rateHeader)
	w.Header().Add("X-RateLimit-Limit", fmt.Sprintf("%d", l.burst))
	w.Header().Add("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Add("X-RateLimit-Reset", fmt.Sprintf("%d", reset))
	l.next.ServeHTTP(w, r)
}
