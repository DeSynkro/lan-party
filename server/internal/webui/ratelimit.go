package webui

import (
	"sync"
	"time"
)

const (
	maxLoginFailures = 5
	maxBackoff       = 15 * time.Minute
	baseBackoff      = 30 * time.Second
	globalFailWindow = time.Minute
	globalFailCap    = 100
)

type loginFailure struct {
	failures     int
	blockedUntil time.Time
	last         time.Time
}

// LoginLimiter slows repeated failed logins per address and overall.
type LoginLimiter struct {
	mu     sync.Mutex
	byIP   map[string]*loginFailure
	global []time.Time
}

// NewLoginLimiter creates a limiter with no failures recorded.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{byIP: map[string]*loginFailure{}}
}

// Allow reports whether ip may try to log in, and how long to wait if not.
func (l *LoginLimiter) Allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneGlobalLocked(now)
	if len(l.global) >= globalFailCap {
		wait := l.global[0].Add(globalFailWindow).Sub(now)
		if wait > 0 {
			return false, wait
		}
	}
	entry, ok := l.byIP[ip]
	if !ok {
		return true, 0
	}
	if entry.blockedUntil.After(now) {
		return false, entry.blockedUntil.Sub(now)
	}
	return true, 0
}

// Fail records one failed login for ip.
func (l *LoginLimiter) Fail(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneGlobalLocked(now)
	l.global = append(l.global, now)

	entry, ok := l.byIP[ip]
	if !ok {
		entry = &loginFailure{}
		l.byIP[ip] = entry
	}
	entry.failures++
	entry.last = now
	if entry.failures >= maxLoginFailures {
		wait := baseBackoff << (entry.failures - maxLoginFailures)
		if wait <= 0 || wait > maxBackoff {
			wait = maxBackoff
		}
		entry.blockedUntil = now.Add(wait)
	}
	l.pruneEntriesLocked(now)
}

// Success forgets the failures recorded for ip.
func (l *LoginLimiter) Success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIP, ip)
}

func (l *LoginLimiter) pruneGlobalLocked(now time.Time) {
	cutoff := now.Add(-globalFailWindow)
	kept := l.global[:0]
	for _, t := range l.global {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.global = kept
}

func (l *LoginLimiter) pruneEntriesLocked(now time.Time) {
	if len(l.byIP) < 1024 {
		return
	}
	for ip, entry := range l.byIP {
		if entry.blockedUntil.After(now) {
			continue
		}
		if now.Sub(entry.last) > time.Hour {
			delete(l.byIP, ip)
		}
	}
}
