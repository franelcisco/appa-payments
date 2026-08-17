package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const (
	otpTTL          = 2 * time.Minute
	otpCleanupEvery = 5 * time.Minute
)

type otpEntry struct {
	code      string
	expiresAt time.Time
}

type otpCache struct {
	mu      sync.Mutex
	entries map[string]otpEntry
}

func newOTPCache() *otpCache {
	c := &otpCache{entries: make(map[string]otpEntry)}
	go c.cleanupLoop()
	return c
}

// Set stores a new OTP for the given key, overwriting any existing entry.
func (c *otpCache) Set(key, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = otpEntry{code: code, expiresAt: time.Now().Add(otpTTL)}
}

// Validate checks that the OTP for key matches code and has not expired.
// On success the entry is consumed (deleted) so it cannot be reused.
func (c *otpCache) Validate(key, code string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return false
	}
	if entry.code != code {
		return false
	}
	delete(c.entries, key)
	return true
}

func (c *otpCache) cleanupLoop() {
	ticker := time.NewTicker(otpCleanupEvery)
	defer ticker.Stop()
	for range ticker.C {
		c.cleanup()
	}
}

func (c *otpCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}
}

// generateOTPCode returns a cryptographically random 6-digit zero-padded code.
func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
