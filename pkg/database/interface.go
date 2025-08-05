package database

import (
	"time"
)

// User represents a user in the database
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Created      time.Time `json:"created"`
	LastLogin    time.Time `json:"last_login"`
	PasskeyID    string    `json:"passkey_id,omitempty"`
	PublicKey    string    `json:"public_key,omitempty"`
	IsRegistered bool      `json:"is_registered"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Created   time.Time `json:"created"`
	ExpiresAt time.Time `json:"expires_at"`
}

// WebAuthnCredential represents a WebAuthn credential
type WebAuthnCredential struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	CredentialID string    `json:"credential_id"`
	PublicKey    string    `json:"public_key"`
	Created      time.Time `json:"created"`
}

// Database interface defines the contract for database operations
type Database interface {
	// User operations
	CreateUser(username string) (*User, error)
	GetUser(username string) (*User, error)
	GetUserByID(id int) (*User, error)
	GetAllUsers() ([]*User, error)
	UpdateUserLastLogin(username string) error
	UpdateUserPasskeyID(username, passkeyID string) error
	UpdateUserPublicKey(username, publicKey string) error
	SetUserRegistered(username string, registered bool) error
	FindUserByPasskeyID(passkeyID string) (*User, error)

	// Session operations
	CreateSession(sessionID, username string) error
	GetSession(sessionID string) (*Session, error)
	DeleteSession(sessionID string) error
	CleanupExpiredSessions() error

	// WebAuthn operations
	StoreCredential(userID int, credentialID, publicKey string) error
	GetCredential(credentialID string) (*WebAuthnCredential, error)
	GetCredentialsByUserID(userID int) ([]*WebAuthnCredential, error)

	// Utility operations
	Close() error
	Init() error
}
