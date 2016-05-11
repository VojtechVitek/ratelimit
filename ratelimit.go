package ratelimit

import (
	"fmt"
	"net/http"
	"time"
)

// TokenBucketStore represents storage implementing Token Bucket algorithm.
type TokenBucketStore interface {
	Take(key string) (taken bool, remaining int, err error)
}

// ResetTime is a TokenBucketStore implementation capable of returning
// timestamp of next expected reset time (next available token).
type HasResetTime interface {
	TokenBucketStore

	// TODO: Do we need "key" parameter too? Maybe we do.
	ResetTime() time.Time
}

type KeyFn func(r *http.Request) string

func Middleware(keyFn KeyFn, rate int, window time.Duration) func(http.Handler) http.Handler {
	rateLimiter := rateLimiter{
		store:       InMemory(rate, window),
		keyFn:       keyFn,
		rate:        rate,
		window:      window,
		rateHeader:  fmt.Sprintf("%d req/min", rate*int(window/time.Minute)),
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
	store       TokenBucketStore
	keyFn       KeyFn
	rate        int
	window      time.Duration
	rateHeader  string
	resetHeader string
}

// ServeHTTPC implements http.Handler interface.
func (l *rateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := l.keyFn(r)
	ok, remaining, err := l.store.Take(key)
	if err != nil {
		// TODO: Fallback stores?
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	if !ok {
		if s, ok := l.store.(HasResetTime); ok {
			w.Header().Add("Retry-After", s.ResetTime().Format(http.TimeFormat))
		}
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	w.Header().Add("X-RateLimit-Key", key)
	w.Header().Add("X-RateLimit-Rate", l.rateHeader)
	w.Header().Add("X-RateLimit-Limit", fmt.Sprintf("%d", l.rate))
	w.Header().Add("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	if s, ok := l.store.(HasResetTime); ok {
		t := s.ResetTime()
		w.Header().Add("X-RateLimit-Reset", fmt.Sprintf("%d", t.Unix()))
		w.Header().Add("Retry-After", t.Format(http.TimeFormat))
	}
	l.next.ServeHTTP(w, r)
}
