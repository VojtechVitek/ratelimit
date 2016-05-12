package ratelimit

import (
	"net/http"
	"time"
)

// TokenBucketStore is an interface for for any storage implementing
// Token Bucket algorithm.
type TokenBucketStore interface {
	InitRate(rate int, window time.Duration)
	Take(key string) (taken bool, remaining int, err error)
}

// HasResetTime is a TokenBucketStore implementation capable of returning
// timestamp of next expected reset time (next available token).
type HasResetTime interface {
	TokenBucketStore

	// TODO: Do we need "key" parameter too? Maybe we do.
	ResetTime() time.Time
}

// KeyFn is a function returning bucket key depending on request data.
type KeyFn func(r *http.Request) string
