package middleware

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type clientLimit struct {
	count     int
	resetTime time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimit
	limit   int
	window  time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientLimit),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		rl.mu.Lock()
		client, exists := rl.clients[ip]
		now := time.Now()

		if !exists || now.After(client.resetTime) {
			rl.clients[ip] = &clientLimit{
				count:     1,
				resetTime: now.Add(rl.window),
			}
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		client.count++
		if client.count > rl.limit {
			rl.mu.Unlock()
			log.Printf("[RATE LIMIT EXCEEDED] IP %s melebihi batas request (%d/%v)", ip, rl.limit, rl.window)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Batas percobaan terlampaui. Silakan tunggu beberapa saat lagi."}`))
			return
		}
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(rl.window)
		rl.mu.Lock()
		now := time.Now()
		for ip, client := range rl.clients {
			if now.After(client.resetTime) {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}
