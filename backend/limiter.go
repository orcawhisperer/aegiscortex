package aegiscortex

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxLimiterIdentities = 4096

type ipLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{hits: map[string][]time.Time{}}
}

func (l *ipLimiter) allow(ip string, n int, window time.Duration) bool {
	if l == nil || ip == "" {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-window)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.hits[ip]; !exists {
		l.evictIfNeededLocked(cutoff)
	}
	kept := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= n {
		l.hits[ip] = kept
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}

func (l *ipLimiter) evictIfNeededLocked(cutoff time.Time) {
	if len(l.hits) < maxLimiterIdentities {
		return
	}
	for k, ts := range l.hits {
		stale := true
		for _, t := range ts {
			if t.After(cutoff) {
				stale = false
				break
			}
		}
		if stale {
			delete(l.hits, k)
			if len(l.hits) < maxLimiterIdentities {
				return
			}
		}
	}
	for k := range l.hits {
		delete(l.hits, k)
		return
	}
}

func socketIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// requestIP is the rate-limit identity. On Vercel the platform sets
// X-Forwarded-For; anywhere else the socket peer is the only trusted source.
func requestIP(r *http.Request) string {
	if HostedOnVercel() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	return socketIP(r)
}

func isLoopbackIP(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackRemote(r *http.Request) bool {
	return isLoopbackIP(socketIP(r))
}

func listenIsLoopback(addr string) bool {
	if addr == "" {
		return true
	}
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:") {
		return true
	}
	if strings.HasPrefix(addr, ":") {
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return isLoopbackIP(host)
}

func allowBrowserKeyHold(r *http.Request) bool {
	if HostedOnVercel() {
		return false
	}
	if !listenIsLoopback(ResolvedListen()) {
		return false
	}
	return isLoopbackRemote(r)
}

func acceptJSONPost(r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(ct, "application/json") {
		return false
	}
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	return true
}
