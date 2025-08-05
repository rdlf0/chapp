package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDB implements the Database interface using SQLite
type SQLiteDB struct {
	db *sql.DB
}

// NewSQLite creates a new SQLite database connection
func NewSQLite(dbPath string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	sqliteDB := &SQLiteDB{db: db}

	// Initialize the database with tables
	if err := sqliteDB.Init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	return sqliteDB, nil
}

// Init creates the database tables
func (s *SQLiteDB) Init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			passkey_id TEXT,
			public_key TEXT,
			is_registered BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP DEFAULT (datetime('now', '+24 hours')),
			FOREIGN KEY (user_id) REFERENCES users (id)
		)`,
		`CREATE TABLE IF NOT EXISTS webauthn_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			credential_id TEXT UNIQUE NOT NULL,
			public_key TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_user_id ON webauthn_credentials(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_credential_id ON webauthn_credentials(credential_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %v", err)
		}
	}

	log.Println("Database initialized successfully")
	return nil
}

// CreateUser creates a new user
func (s *SQLiteDB) CreateUser(username string) (*User, error) {
	query := `INSERT INTO users (username, created_at, last_login) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	result, err := s.db.Exec(query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %v", err)
	}

	return s.GetUserByID(int(id))
}

// GetUser retrieves a user by username
func (s *SQLiteDB) GetUser(username string) (*User, error) {
	query := `SELECT id, username, created_at, last_login, passkey_id, public_key, is_registered 
			  FROM users WHERE username = ?`

	var user User
	var passkeyID, publicKey sql.NullString
	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Created,
		&user.LastLogin,
		&passkeyID,
		&publicKey,
		&user.IsRegistered,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Handle NULL values
	if passkeyID.Valid {
		user.PasskeyID = passkeyID.String
	}
	if publicKey.Valid {
		user.PublicKey = publicKey.String
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *SQLiteDB) GetUserByID(id int) (*User, error) {
	query := `SELECT id, username, created_at, last_login, passkey_id, public_key, is_registered 
			  FROM users WHERE id = ?`

	var user User
	var passkeyID, publicKey sql.NullString
	err := s.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Created,
		&user.LastLogin,
		&passkeyID,
		&publicKey,
		&user.IsRegistered,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %v", err)
	}

	// Handle NULL values
	if passkeyID.Valid {
		user.PasskeyID = passkeyID.String
	}
	if publicKey.Valid {
		user.PublicKey = publicKey.String
	}

	return &user, nil
}

// GetAllUsers retrieves all users from the database
func (s *SQLiteDB) GetAllUsers() ([]*User, error) {
	query := `SELECT id, username, created_at, last_login, passkey_id, public_key, is_registered 
			  FROM users ORDER BY username`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %v", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var passkeyID, publicKey sql.NullString
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Created,
			&user.LastLogin,
			&passkeyID,
			&publicKey,
			&user.IsRegistered,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}

		// Handle NULL values
		if passkeyID.Valid {
			user.PasskeyID = passkeyID.String
		}
		if publicKey.Valid {
			user.PublicKey = publicKey.String
		}

		users = append(users, &user)
	}

	return users, nil
}

// UpdateUserLastLogin updates the user's last login time
func (s *SQLiteDB) UpdateUserLastLogin(username string) error {
	query := `UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE username = ?`

	result, err := s.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to update last login: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// UpdateUserPasskeyID updates the user's passkey ID
func (s *SQLiteDB) UpdateUserPasskeyID(username, passkeyID string) error {
	query := `UPDATE users SET passkey_id = ? WHERE username = ?`

	result, err := s.db.Exec(query, passkeyID, username)
	if err != nil {
		return fmt.Errorf("failed to update passkey ID: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// UpdateUserPublicKey updates the user's public key
func (s *SQLiteDB) UpdateUserPublicKey(username, publicKey string) error {
	query := `UPDATE users SET public_key = ? WHERE username = ?`

	result, err := s.db.Exec(query, publicKey, username)
	if err != nil {
		return fmt.Errorf("failed to update public key: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// SetUserRegistered sets the user's registration status
func (s *SQLiteDB) SetUserRegistered(username string, registered bool) error {
	query := `UPDATE users SET is_registered = ? WHERE username = ?`

	result, err := s.db.Exec(query, registered, username)
	if err != nil {
		return fmt.Errorf("failed to set user registered: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// FindUserByPasskeyID finds a user by their passkey ID
func (s *SQLiteDB) FindUserByPasskeyID(passkeyID string) (*User, error) {
	query := `SELECT id, username, created_at, last_login, passkey_id, public_key, is_registered 
			  FROM users WHERE passkey_id = ?`

	var user User
	var userPasskeyID, publicKey sql.NullString
	err := s.db.QueryRow(query, passkeyID).Scan(
		&user.ID,
		&user.Username,
		&user.Created,
		&user.LastLogin,
		&userPasskeyID,
		&publicKey,
		&user.IsRegistered,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by passkey ID: %v", err)
	}

	// Handle NULL values
	if userPasskeyID.Valid {
		user.PasskeyID = userPasskeyID.String
	}
	if publicKey.Valid {
		user.PublicKey = publicKey.String
	}

	return &user, nil
}

// CreateSession creates a new session
func (s *SQLiteDB) CreateSession(sessionID, username string) error {
	// First get the user ID
	user, err := s.GetUser(username)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}

	query := `INSERT INTO sessions (id, user_id, username, created_at, expires_at) 
			  VALUES (?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '+24 hours'))`

	_, err = s.db.Exec(query, sessionID, user.ID, username)
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}

	return nil
}

// GetSession retrieves a session by ID
func (s *SQLiteDB) GetSession(sessionID string) (*Session, error) {
	query := `SELECT id, user_id, username, created_at, expires_at FROM sessions WHERE id = ?`

	var session Session
	err := s.db.QueryRow(query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.Username,
		&session.Created,
		&session.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %v", err)
	}

	return &session, nil
}

// DeleteSession deletes a session
func (s *SQLiteDB) DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`

	result, err := s.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions
func (s *SQLiteDB) CleanupExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP`

	result, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected > 0 {
		log.Printf("Cleaned up %d expired sessions", rowsAffected)
	}

	return nil
}

// StoreCredential stores a WebAuthn credential
func (s *SQLiteDB) StoreCredential(userID int, credentialID, publicKey string) error {
	query := `INSERT INTO webauthn_credentials (user_id, credential_id, public_key, created_at) 
			  VALUES (?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := s.db.Exec(query, userID, credentialID, publicKey)
	if err != nil {
		return fmt.Errorf("failed to store credential: %v", err)
	}

	return nil
}

// GetCredential retrieves a WebAuthn credential by ID
func (s *SQLiteDB) GetCredential(credentialID string) (*WebAuthnCredential, error) {
	query := `SELECT id, user_id, credential_id, public_key, created_at 
			  FROM webauthn_credentials WHERE credential_id = ?`

	var cred WebAuthnCredential
	err := s.db.QueryRow(query, credentialID).Scan(
		&cred.ID,
		&cred.UserID,
		&cred.CredentialID,
		&cred.PublicKey,
		&cred.Created,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %v", err)
	}

	return &cred, nil
}

// GetCredentialsByUserID retrieves all credentials for a user
func (s *SQLiteDB) GetCredentialsByUserID(userID int) ([]*WebAuthnCredential, error) {
	query := `SELECT id, user_id, credential_id, public_key, created_at 
			  FROM webauthn_credentials WHERE user_id = ?`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %v", err)
	}
	defer rows.Close()

	var credentials []*WebAuthnCredential
	for rows.Next() {
		var cred WebAuthnCredential
		err := rows.Scan(
			&cred.ID,
			&cred.UserID,
			&cred.CredentialID,
			&cred.PublicKey,
			&cred.Created,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential: %v", err)
		}
		credentials = append(credentials, &cred)
	}

	return credentials, nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}
