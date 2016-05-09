package ratelimiter

import (
	"sync"
	"time"
)

type token struct{}

type memoryStore struct {
	sync.Mutex // guards buckets
	buckets    map[string]chan token
	bucketLen  int
	reset      time.Time
}

func InMemory(rate int, window time.Duration) *memoryStore {
	s := memoryStore{
		buckets:   map[string]chan token{},
		bucketLen: rate,
		reset:     time.Now(),
	}
	go func() {
		interval := time.Duration(int(window) / rate)
		tick := time.NewTicker(interval)
		for t := range tick.C {
			s.Lock()
			s.reset = t.Add(interval)
			for key, bucket := range s.buckets {
				select {
				case <-bucket:
				default:
					delete(s.buckets, key)
				}
			}
			s.Unlock()
		}
	}()
	return &s
}

// Take implements TokenBucketStore interface.
func (s *memoryStore) Take(key string) (bool, int, error) {
	s.Lock()
	bucket, ok := s.buckets[key]
	if !ok {
		bucket = make(chan token, s.bucketLen)
		s.buckets[key] = bucket
	}
	s.Unlock()
	select {
	case bucket <- token{}:
		return true, cap(bucket) - len(bucket), nil
	default:
		return false, 0, nil
	}
}

func (s *memoryStore) ResetTime() time.Time {
	return s.reset
}
