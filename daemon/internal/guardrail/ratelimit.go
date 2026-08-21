package guardrail

import (
	"sync"
	"time"
)

// RateLimiterConfig tunes exec request throttling and anomaly detection.
type RateLimiterConfig struct {
	BurstSize     int           // max requests in a burst window
	BurstWindow   time.Duration // sliding window for the burst
	Cooldown      time.Duration // penalty applied after anomaly detection
	MaxDeniedRate int           // denied requests within window that trigger cooldown
}

// DefaultRateLimit returns sane defaults: 20 req / 10s, 30s cooldown,
// 5 denials trigger anomaly mode.
func DefaultRateLimit() RateLimiterConfig {
	return RateLimiterConfig{
		BurstSize:     20,
		BurstWindow:   10 * time.Second,
		Cooldown:      30 * time.Second,
		MaxDeniedRate: 5,
	}
}

type clientState struct {
	timestamps []time.Time
	denied     []time.Time
	blockedTil time.Time
}

// RateLimiter tracks per-client (e.g. per WS connection id) exec frequency
// and applies escalating cooldowns when anomalous behaviour is detected:
// bursts of requests or repeated blacklist denials.
type RateLimiter struct {
	cfg RateLimiterConfig

	mu    sync.Mutex
	state map[string]*clientState
}

// NewRateLimiter constructs a limiter with cfg.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.BurstSize <= 0 || cfg.BurstWindow <= 0 {
		cfg = DefaultRateLimit()
	}
	return &RateLimiter{cfg: cfg, state: make(map[string]*clientState)}
}

// Allow reports whether client may execute now. It also records denial
// feedback via RecordDenied to drive anomaly detection.
func (r *RateLimiter) Allow(clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.state[clientID]
	if st == nil {
		st = &clientState{}
		r.state[clientID] = st
	}

	now := time.Now()
	if now.Before(st.blockedTil) {
		return false
	}
	r.prune(st, now)

	if len(st.timestamps) >= r.cfg.BurstSize {
		st.blockedTil = now.Add(r.cfg.Cooldown)
		st.timestamps = nil
		return false
	}
	st.timestamps = append(st.timestamps, now)
	return true
}

// RecordDenied feeds a blacklist/classification denial into anomaly stats.
func (r *RateLimiter) RecordDenied(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.state[clientID]
	if st == nil {
		st = &clientState{}
		r.state[clientID] = st
	}
	now := time.Now()
	r.prune(st, now)
	st.denied = append(st.denied, now)

	if len(st.denied) >= r.cfg.MaxDeniedRate {
		st.blockedTil = now.Add(2 * r.cfg.Cooldown) // escalate
		st.denied = nil
	}
}

// BlockedUntil reports remaining cooldown (zero if not blocked).
func (r *RateLimiter) BlockedUntil(clientID string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st := r.state[clientID]; st != nil {
		return st.blockedTil
	}
	return time.Time{}
}

// prune drops timestamps outside the burst window.
func (r *RateLimiter) prune(st *clientState, now time.Time) {
	cutoff := now.Add(-r.cfg.BurstWindow)
	kept := st.timestamps[:0]
	for _, t := range st.timestamps {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	st.timestamps = kept

	dkept := st.denied[:0]
	for _, t := range st.denied {
		if t.After(cutoff) {
			dkept = append(dkept, t)
		}
	}
	st.denied = dkept
}