package auth

import (
	"errors"
	"fmt"
	"gcli2apigo/internal/banlist"
	"sync"
	"time"
)

// RateLimitedCredentialPool extends CredentialPool with rate limiting per credential
type RateLimitedCredentialPool struct {
	*CredentialPool
	lastUsed     map[string]time.Time // Track last usage time per project ID
	minInterval  time.Duration        // Minimum time between uses of same credential
	currentIndex int                  // For round-robin selection
	mu           sync.RWMutex
}

// NewRateLimitedCredentialPool creates a new rate-limited credential pool
// minInterval: minimum time between consecutive uses of the same credential (e.g., 100ms for 10 RPS per credential)
func NewRateLimitedCredentialPool(minInterval time.Duration) *RateLimitedCredentialPool {
	return &RateLimitedCredentialPool{
		CredentialPool: NewCredentialPool(),
		lastUsed:       make(map[string]time.Time),
		minInterval:    minInterval,
		currentIndex:   0,
	}
}

// GetCredentialWithRateLimit returns the next credential using round-robin with rate limiting
// If a credential was used too recently, it skips to the next one
func (rlcp *RateLimitedCredentialPool) GetCredentialWithRateLimit() (*CredentialEntry, error) {
	rlcp.mu.Lock()
	defer rlcp.mu.Unlock()

	if len(rlcp.credentials) == 0 {
		return nil, errors.New("no credentials available in pool")
	}

	// Filter out banned credentials
	bl := banlist.GetBanList()
	availableCredentials := make([]*CredentialEntry, 0, len(rlcp.credentials))

	for _, cred := range rlcp.credentials {
		if !bl.IsBanned(cred.ProjectID) {
			availableCredentials = append(availableCredentials, cred)
		}
	}

	if len(availableCredentials) == 0 {
		return nil, errors.New("no unbanned credentials available in pool")
	}

	// Try to find a credential that hasn't been used recently
	now := time.Now()
	attempts := 0
	maxAttempts := len(availableCredentials) * 2 // Allow wrapping around twice

	for attempts < maxAttempts {
		// Get next credential in round-robin fashion
		cred := availableCredentials[rlcp.currentIndex%len(availableCredentials)]
		rlcp.currentIndex = (rlcp.currentIndex + 1) % len(availableCredentials)
		attempts++

		// Check if this credential can be used (not used too recently)
		lastUsedTime, exists := rlcp.lastUsed[cred.ProjectID]
		if !exists || now.Sub(lastUsedTime) >= rlcp.minInterval {
			// Update last used time
			rlcp.lastUsed[cred.ProjectID] = now
			return cred, nil
		}

		// If we've tried all credentials and none are available, wait for the shortest time
		if attempts >= len(availableCredentials) {
			// Find the credential that will be available soonest
			var shortestWait time.Duration = rlcp.minInterval
			for _, c := range availableCredentials {
				if lastTime, ok := rlcp.lastUsed[c.ProjectID]; ok {
					wait := rlcp.minInterval - now.Sub(lastTime)
					if wait > 0 && wait < shortestWait {
						shortestWait = wait
					}
				}
			}

			// Sleep for the shortest wait time
			if shortestWait > 0 {
				fmt.Printf("[DEBUG] All credentials recently used, waiting %v before retry\n", shortestWait)
				time.Sleep(shortestWait)
				now = time.Now()
				attempts = 0 // Reset attempts after waiting
			}
		}
	}

	// Fallback: return any available credential (shouldn't reach here normally)
	cred := availableCredentials[0]
	rlcp.lastUsed[cred.ProjectID] = now
	return cred, nil
}

// ResetRateLimits clears all rate limit tracking (useful for testing or manual reset)
func (rlcp *RateLimitedCredentialPool) ResetRateLimits() {
	rlcp.mu.Lock()
	defer rlcp.mu.Unlock()

	rlcp.lastUsed = make(map[string]time.Time)
	rlcp.currentIndex = 0
}

// GetMinInterval returns the configured minimum interval between credential uses
func (rlcp *RateLimitedCredentialPool) GetMinInterval() time.Duration {
	return rlcp.minInterval
}

// SetMinInterval updates the minimum interval between credential uses
func (rlcp *RateLimitedCredentialPool) SetMinInterval(interval time.Duration) {
	rlcp.mu.Lock()
	defer rlcp.mu.Unlock()

	rlcp.minInterval = interval
}
