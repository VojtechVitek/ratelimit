package redis

import (
	"time"

	"github.com/garyburd/redigo/redis"
)

var PrefixKey = "ratelimit:"

type bucketStore struct {
	pool *redis.Pool

	rate          int
	windowSeconds int
}

// New creates new in-memory token bucket store.
func New(pool *redis.Pool) *bucketStore {
	return &bucketStore{
		pool: pool,
	}
}

func (s *bucketStore) InitRate(rate int, window time.Duration) {
	s.rate = rate
	s.windowSeconds = int(window / time.Second)
	if s.windowSeconds <= 1 {
		s.windowSeconds = 1
	}
}

// Take implements TokenBucketStore interface. It takes token from a bucket
// referenced by a given key, if available.
func (s *bucketStore) Take(key string) (bool, int, error) {
	c := s.pool.Get()
	defer c.Close()

	// Number of tokens in the bucket.
	bucketLen, err := redis.Int(c.Do("LLEN", PrefixKey+key))
	if err != nil {
		return false, 0, err
	}

	// Bucket is full.
	if bucketLen >= s.rate {
		return false, 0, nil
	}

	if bucketLen > 0 {
		// Bucket most probably exists, try to push a new token into it.
		// If RPUSHX returns 0 (ie. key expired between LLEN and RPUSHX), we need
		// to fall-back to RPUSH without returning error.
		c.Send("MULTI")
		c.Send("RPUSHX", PrefixKey+key, "")
		reply, err := redis.Ints(c.Do("EXEC"))
		if err != nil {
			return false, 0, err
		}
		bucketLen = reply[0]
		if bucketLen > 0 {
			return true, s.rate - bucketLen - 1, nil
		}
	}

	c.Send("MULTI")
	c.Send("RPUSH", PrefixKey+key, "")
	c.Send("EXPIRE", PrefixKey+key, s.windowSeconds)
	if _, err := c.Do("EXEC"); err != nil {
		return false, 0, err
	}

	return true, s.rate - bucketLen - 1, nil
}

// TODO: Make this return value of Take(key) instead.
// TODO: Return int (remaining seconds) to prevent time-sync issues?
func (s *bucketStore) ResetTime() time.Time {
	c := s.pool.Get()
	defer c.Close()

	// TODO: Doesn't really work without given key.
	ttl, err := redis.Int(c.Do("TTL", PrefixKey+"key"))
	if err != nil || ttl < 0 {
		return time.Now()
	}

	return time.Now().Add(time.Duration(ttl) * time.Second)
}
