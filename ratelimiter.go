package ratelimiter

import (
	"net/http"
	"time"
)

type KeyFn func(r *http.Request) string

func Middleware(keyFn KeyFn, rate time.Duration, burstLimit int) func(http.Handler) http.Handler {
	l := inMemoryRateLimiter{
		rate:       rate,
		keyFn:      keyFn,
		burstLimit: burstLimit,
		buckets:    map[string]chan token{},
	}
	l.Run()

	fn := func(h http.Handler) http.Handler {
		l.next = h
		return &l
	}
	return fn
}

type token struct{}

type inMemoryRateLimiter struct {
	next       http.Handler
	keyFn      KeyFn
	rate       time.Duration
	burstLimit int
	buckets    map[string]chan token
}

func (l *inMemoryRateLimiter) Run() {
	go func() {
		tick := time.NewTicker(l.rate)
		for range tick.C {
			for key, bucket := range l.buckets {
				select {
				case <-bucket:
				default:
					delete(l.buckets, key)
				}
			}
		}
	}()
}

// ServeHTTPC implements http.Handler interface.
func (l *inMemoryRateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := l.keyFn(r)
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = make(chan token, l.burstLimit)
		l.buckets[key] = bucket
	}
	select {
	case bucket <- token{}:
		l.next.ServeHTTP(w, r)
	default:
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
}
