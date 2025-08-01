package auth

import (
	"testing"
)

// TestCreateSession tests session creation
func TestCreateSession(t *testing.T) {
	username := "testuser"
	sessionID := CreateSession(username)

	if sessionID == "" {
		t.Error("Session ID should not be empty")
	}

	// Test that session was actually created
	session := GetSession(sessionID)
	if session == nil {
		t.Error("Session should exist after creation")
	}

	if session.Username != username {
		t.Errorf("Expected username '%s', got '%s'", username, session.Username)
	}

	if session.Created.IsZero() {
		t.Error("Session should have a creation time")
	}
}

// TestGetSession tests session retrieval
func TestGetSession(t *testing.T) {
	// Test with non-existent session
	session := GetSession("nonexistent")
	if session != nil {
		t.Error("Non-existent session should return nil")
	}

	// Test with valid session
	username := "testuser"
	sessionID := CreateSession(username)
	session = GetSession(sessionID)

	if session == nil {
		t.Error("Valid session should not return nil")
	}

	if session.Username != username {
		t.Errorf("Expected username '%s', got '%s'", username, session.Username)
	}
}

// TestDeleteSession tests session deletion
func TestDeleteSession(t *testing.T) {
	// Create a session
	username := "testuser"
	sessionID := CreateSession(username)

	// Verify it exists
	session := GetSession(sessionID)
	if session == nil {
		t.Fatal("Session should exist before deletion")
	}

	// Delete the session
	DeleteSession(sessionID)

	// Verify it's gone
	session = GetSession(sessionID)
	if session != nil {
		t.Error("Session should not exist after deletion")
	}
}

// TestMultipleSessions tests multiple session creation
func TestMultipleSessions(t *testing.T) {
	// Create multiple sessions for different users
	users := []string{"user1", "user2", "user3"}
	sessionIDs := make([]string, len(users))

	for i, username := range users {
		sessionIDs[i] = CreateSession(username)
		if sessionIDs[i] == "" {
			t.Errorf("Session ID should not be empty for user %s", username)
		}
	}

	// Verify all sessions exist
	for i, sessionID := range sessionIDs {
		session := GetSession(sessionID)
		if session == nil {
			t.Errorf("Session should exist for user %s", users[i])
		}
		if session.Username != users[i] {
			t.Errorf("Expected username '%s', got '%s'", users[i], session.Username)
		}
	}
}

// TestSessionUniqueness tests that session IDs are unique
func TestSessionUniqueness(t *testing.T) {
	username := "testuser"
	sessionID1 := CreateSession(username)
	sessionID2 := CreateSession(username)

	if sessionID1 == sessionID2 {
		t.Error("Session IDs should be unique")
	}

	// Both sessions should exist
	session1 := GetSession(sessionID1)
	session2 := GetSession(sessionID2)

	if session1 == nil || session2 == nil {
		t.Error("Both sessions should exist")
	}
}
