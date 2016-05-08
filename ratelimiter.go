package ratelimiter

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type KeyFn func(r *http.Request) string

func Middleware(keyFn KeyFn, rate time.Duration, burst int) func(http.Handler) http.Handler {
	if burst < 1 {
		burst = 1
	}
	l := inMemoryRateLimiter{
		rate:        rate,
		keyFn:       keyFn,
		burst:       burst,
		buckets:     map[string]chan token{},
		rateHeader:  fmt.Sprintf("%d req/min", time.Minute/rate),
		resetHeader: fmt.Sprintf("%d", time.Now().Unix()),
	}
	go l.Run()

	fn := func(h http.Handler) http.Handler {
		l.next = h
		return &l
	}
	return fn
}

type token struct{}

type inMemoryRateLimiter struct {
	next        http.Handler
	keyFn       KeyFn
	rate        time.Duration
	burst       int
	sync.Mutex  // guards buckets map
	buckets     map[string]chan token
	rateHeader  string
	resetHeader string
}

func (l *inMemoryRateLimiter) Run() {
	tick := time.NewTicker(l.rate)
	for t := range tick.C {
		l.Lock()
		l.resetHeader = fmt.Sprintf("%d", t.Add(l.rate).Unix())
		for key, bucket := range l.buckets {
			select {
			case <-bucket:
			default:
				delete(l.buckets, key)
			}
		}
		l.Unlock()
	}
}

// ServeHTTPC implements http.Handler interface.
func (l *inMemoryRateLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := l.keyFn(r)
	l.Lock()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = make(chan token, l.burst)
		l.buckets[key] = bucket
	}
	l.Unlock()
	select {
	case bucket <- token{}:
		w.Header().Add("X-RateLimit-Key", key)
		w.Header().Add("X-RateLimit-Rate", l.rateHeader)
		w.Header().Add("X-RateLimit-Limit", fmt.Sprintf("%d", cap(bucket)))
		w.Header().Add("X-RateLimit-Remaining", fmt.Sprintf("%d", cap(bucket)-len(bucket)))
		w.Header().Add("X-RateLimit-Reset", l.resetHeader)
		l.next.ServeHTTP(w, r)
	default:
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
}
