package dashboard

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session represents a user authentication session
type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager manages user authentication sessions
// Security features:
// - Cryptographically secure session IDs using UUID v4
// - Automatic expiration after 24 hours
// - Background cleanup of expired sessions every hour
// - Thread-safe operations using RWMutex
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new SessionManager instance
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
	}

	// Start automatic cleanup goroutine
	go sm.cleanupLoop()

	return sm
}

// CreateSession creates a new session and returns it
func (sm *SessionManager) CreateSession() (*Session, error) {
	// Generate a cryptographically secure session ID using UUID
	sessionID := uuid.New().String()

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // 24-hour expiration
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	log.Printf("[DEBUG] Created new session: %s (expires: %v)", sessionID, session.ExpiresAt)
	return session, nil
}

// ValidateSession checks if a session ID is valid and not expired
func (sm *SessionManager) ValidateSession(sessionID string) bool {
	if sessionID == "" {
		log.Printf("[DEBUG] Session validation failed: empty session ID")
		return false
	}

	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		log.Printf("[DEBUG] Session validation failed: session not found: %s", sessionID)
		return false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		log.Printf("[INFO] Session expired: %s (expired at: %v)", sessionID, session.ExpiresAt)
		// Clean up expired session
		sm.DeleteSession(sessionID)
		return false
	}

	log.Printf("[DEBUG] Session validated successfully: %s", sessionID)
	return true
}

// DeleteSession removes a session from the manager
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()
	log.Printf("[DEBUG] Session deleted: %s", sessionID)
}

// CleanupExpiredSessions removes all expired sessions
func (sm *SessionManager) CleanupExpiredSessions() {
	now := time.Now()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	cleanedCount := 0
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, id)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		log.Printf("[INFO] Cleaned up %d expired sessions", cleanedCount)
	}
}

// cleanupLoop runs a background goroutine that periodically cleans up expired sessions
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.CleanupExpiredSessions()
	}
}

// generateSecureToken generates a cryptographically secure random token
// This is a helper function that can be used for additional security features
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
