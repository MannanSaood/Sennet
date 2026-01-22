package middleware

import (
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*tokenBucket
	rate     float64
	capacity int
	cleanup  time.Duration
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

func NewRateLimiter(requestsPerMinute int, burstSize int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     float64(requestsPerMinute) / 60.0,
		capacity: burstSize,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			if now.Sub(bucket.lastUpdate) > rl.cleanup {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	now := time.Now()

	if !exists {
		rl.buckets[key] = &tokenBucket{
			tokens:     float64(rl.capacity) - 1,
			lastUpdate: now,
		}
		return true
	}

	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.capacity) {
		bucket.tokens = float64(rl.capacity)
	}
	bucket.lastUpdate = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		if key == "" {
			key = r.RemoteAddr
		}

		if !rl.Allow(key) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
