package auth

import (
	"sync"
	"time"
)

// OnboardingCache caches onboarding state per project to avoid redundant API calls
type OnboardingCache struct {
	cache sync.Map // map[string]*OnboardingEntry
}

// OnboardingEntry represents a cached onboarding state for a project
type OnboardingEntry struct {
	ProjectID   string
	OnboardedAt time.Time
	TTL         time.Duration
}

// NewOnboardingCache creates a new OnboardingCache with default settings
func NewOnboardingCache() *OnboardingCache {
	return &OnboardingCache{
		cache: sync.Map{},
	}
}

// IsOnboarded checks if a project is onboarded and the cache entry is still valid
func (oc *OnboardingCache) IsOnboarded(projectID string) bool {
	if projectID == "" {
		return false
	}

	value, ok := oc.cache.Load(projectID)
	if !ok {
		return false
	}

	entry, ok := value.(*OnboardingEntry)
	if !ok {
		return false
	}

	// Check if the cache entry has expired
	if time.Since(entry.OnboardedAt) > entry.TTL {
		// Entry expired, remove it from cache
		oc.cache.Delete(projectID)
		return false
	}

	return true
}

// MarkOnboarded marks a project as onboarded with a default TTL of 1 hour
func (oc *OnboardingCache) MarkOnboarded(projectID string) {
	if projectID == "" {
		return
	}

	entry := &OnboardingEntry{
		ProjectID:   projectID,
		OnboardedAt: time.Now(),
		TTL:         1 * time.Hour, // Default TTL: 1 hour
	}

	oc.cache.Store(projectID, entry)
}

// Invalidate removes a specific project from the cache
func (oc *OnboardingCache) Invalidate(projectID string) {
	if projectID == "" {
		return
	}

	oc.cache.Delete(projectID)
}

// Clear removes all entries from the cache
func (oc *OnboardingCache) Clear() {
	oc.cache.Range(func(key, value interface{}) bool {
		oc.cache.Delete(key)
		return true
	})
}
