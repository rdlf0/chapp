package database

import (
	"os"
	"testing"
)

func TestSQLiteDatabase(t *testing.T) {
	// Use a temporary database file
	dbPath := "test_chapp.db"
	defer os.Remove(dbPath)

	// Create database
	db, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test user creation
	user, err := db.CreateUser("testuser")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user == nil {
		t.Fatal("User should not be nil")
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}

	if user.ID == 0 {
		t.Error("User ID should not be zero")
	}

	// Test user retrieval
	retrievedUser, err := db.GetUser("testuser")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrievedUser == nil {
		t.Fatal("Retrieved user should not be nil")
	}

	if retrievedUser.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", retrievedUser.Username)
	}

	// Test session creation
	sessionID := "test-session-id"
	err = db.CreateSession(sessionID, "testuser")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test session retrieval
	session, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session == nil {
		t.Fatal("Session should not be nil")
	}

	if session.ID != sessionID {
		t.Errorf("Expected session ID '%s', got '%s'", sessionID, session.ID)
	}

	if session.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", session.Username)
	}

	// Test session deletion
	err = db.DeleteSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session is deleted
	deletedSession, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get deleted session: %v", err)
	}

	if deletedSession != nil {
		t.Error("Session should be nil after deletion")
	}

	// Test user update
	err = db.UpdateUserLastLogin("testuser")
	if err != nil {
		t.Fatalf("Failed to update user last login: %v", err)
	}

	// Test passkey ID update
	err = db.UpdateUserPasskeyID("testuser", "test-passkey-id")
	if err != nil {
		t.Fatalf("Failed to update passkey ID: %v", err)
	}

	// Test finding user by passkey ID
	foundUser, err := db.FindUserByPasskeyID("test-passkey-id")
	if err != nil {
		t.Fatalf("Failed to find user by passkey ID: %v", err)
	}

	if foundUser == nil {
		t.Fatal("User should be found by passkey ID")
	}

	if foundUser.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", foundUser.Username)
	}

	// Test credential storage
	err = db.StoreCredential(user.ID, "test-credential-id", "test-public-key")
	if err != nil {
		t.Fatalf("Failed to store credential: %v", err)
	}

	// Test credential retrieval
	credential, err := db.GetCredential("test-credential-id")
	if err != nil {
		t.Fatalf("Failed to get credential: %v", err)
	}

	if credential == nil {
		t.Fatal("Credential should not be nil")
	}

	if credential.CredentialID != "test-credential-id" {
		t.Errorf("Expected credential ID 'test-credential-id', got '%s'", credential.CredentialID)
	}

	if credential.PublicKey != "test-public-key" {
		t.Errorf("Expected public key 'test-public-key', got '%s'", credential.PublicKey)
	}

	// Test cleanup
	err = db.CleanupExpiredSessions()
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}
}
