package auth

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"time"

	"chapp/cmd/server/types"
	"chapp/pkg/database"
)

// generateSessionID creates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// CreateSession creates a new session for a username
func CreateSession(username string) string {
	sessionID := generateSessionID()

	// Store session in database
	db := database.GetDatabase()
	if db != nil {
		if err := db.CreateSession(sessionID, username); err != nil {
			log.Printf("Failed to create session in database: %v", err)
		}
	}

	// Also keep in memory for backward compatibility
	session := &types.Session{
		Username: username,
		Created:  time.Now(),
	}

	types.SessionMutex.Lock()
	types.Sessions[sessionID] = session
	types.SessionMutex.Unlock()

	return sessionID
}

// GetSession retrieves a session by ID
func GetSession(sessionID string) *types.Session {
	// Try database first
	db := database.GetDatabase()
	if db != nil {
		session, err := db.GetSession(sessionID)
		if err != nil {
			log.Printf("Failed to get session from database: %v", err)
		} else if session != nil {
			// Convert database session to types.Session
			return &types.Session{
				Username: session.Username,
				Created:  session.Created,
			}
		}
	}

	// Fallback to memory
	types.SessionMutex.RLock()
	defer types.SessionMutex.RUnlock()
	return types.Sessions[sessionID]
}

// DeleteSession removes a session
func DeleteSession(sessionID string) {
	// Remove from database
	db := database.GetDatabase()
	if db != nil {
		if err := db.DeleteSession(sessionID); err != nil {
			log.Printf("Failed to delete session from database: %v", err)
		}
	}

	// Also remove from memory
	types.SessionMutex.Lock()
	delete(types.Sessions, sessionID)
	types.SessionMutex.Unlock()
}

// cleanupSessions removes old sessions (older than 24 hours)
func cleanupSessions() {
	// Cleanup database sessions
	db := database.GetDatabase()
	if db != nil {
		if err := db.CleanupExpiredSessions(); err != nil {
			log.Printf("Failed to cleanup database sessions: %v", err)
		}
	}

	// Also cleanup memory sessions
	types.SessionMutex.Lock()
	defer types.SessionMutex.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, session := range types.Sessions {
		if session.Created.Before(cutoff) {
			delete(types.Sessions, id)
		}
	}
}

// StartSessionCleanup starts the session cleanup goroutine
func StartSessionCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanupSessions()
		}
	}()
}
