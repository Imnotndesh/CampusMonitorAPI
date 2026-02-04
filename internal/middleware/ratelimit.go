package middleware

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	lastSeen time.Time
	count    int
}

type rateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}

	go rl.cleanup()

	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{
			lastSeen: time.Now(),
			count:    1,
		}
		return true
	}

	if time.Since(v.lastSeen) > rl.window {
		v.count = 1
		v.lastSeen = time.Now()
		return true
	}

	if v.count >= rl.limit {
		return false
	}

	v.count++
	v.lastSeen = time.Now()
	return true
}

func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	rl := newRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "Rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
