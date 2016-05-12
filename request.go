package ratelimit

import (
	"fmt"
	"net/http"
	"time"
)

func Request(keyFn KeyFn) *requestBuilder {
	return &requestBuilder{
		keyFn: keyFn,
	}
}

type requestBuilder struct {
	keyFn       KeyFn
	rate        int
	window      time.Duration
	rateHeader  string
	resetHeader string
}

func (b *requestBuilder) Rate(rate int, window time.Duration) *requestBuilder {
	b.rate = rate
	b.window = window
	b.rateHeader = fmt.Sprintf("%d req/min", rate*int(window/time.Minute))
	b.resetHeader = fmt.Sprintf("%d", time.Now().Unix())
	return b
}

// TODO: Custom burst?
// func (b *requestBuilder) Burst(burst int) *requestBuilder {}

func (b *requestBuilder) LimitBy(store TokenBucketStore, fallbackStores ...TokenBucketStore) func(http.Handler) http.Handler {
	store.InitRate(b.rate, b.window)
	for _, store := range fallbackStores {
		store.InitRate(b.rate, b.window)
	}

	limiter := requestLimiter{
		requestBuilder: b,
		store:          store,
		fallbackStores: fallbackStores,
	}

	fn := func(next http.Handler) http.Handler {
		limiter.next = next
		return &limiter
	}

	return fn
}

type requestLimiter struct {
	*requestBuilder

	next           http.Handler
	store          TokenBucketStore
	fallbackStores []TokenBucketStore
}

// ServeHTTPC implements http.Handler interface.
func (l *requestLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := l.keyFn(r)
	if key == "" {
		l.next.ServeHTTP(w, r)
		return
	}

	ok, remaining, err := l.store.Take("request:" + key)
	if err != nil {
		for _, store := range l.fallbackStores {
			ok, remaining, err = store.Take("request:" + key)
			if err == nil {
				break
			}
		}
		if err != nil {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
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
