package ratelimiter

import (
	"sync"
	"time"
)

type token struct{}

type memoryStore struct {
	sync.Mutex    // guards buckets
	buckets       map[string]chan token
	bucketLen     int
	resetUnixTime int64
}

func MemoryStore(rate time.Duration, burst int) *memoryStore {
	s := memoryStore{
		buckets:       map[string]chan token{},
		bucketLen:     burst,
		resetUnixTime: time.Now().Unix(),
	}
	go func() {
		tick := time.NewTicker(rate)
		for t := range tick.C {
			s.Lock()
			s.resetUnixTime = t.Add(rate).Unix()
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

func (s *memoryStore) Take(key string) (bool, int, int64, error) {
	s.Lock()
	bucket, ok := s.buckets[key]
	if !ok {
		bucket = make(chan token, s.bucketLen)
		s.buckets[key] = bucket
	}
	s.Unlock()
	select {
	case bucket <- token{}:
		return true, cap(bucket) - len(bucket), s.resetUnixTime, nil
	default:
		return false, 0, s.resetUnixTime, nil
	}
}
