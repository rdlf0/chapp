package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"chapp/cmd/server/types"
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
	types.SessionMutex.RLock()
	defer types.SessionMutex.RUnlock()
	return types.Sessions[sessionID]
}

// DeleteSession removes a session
func DeleteSession(sessionID string) {
	types.SessionMutex.Lock()
	delete(types.Sessions, sessionID)
	types.SessionMutex.Unlock()
}

// cleanupSessions removes old sessions (older than 24 hours)
func cleanupSessions() {
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
