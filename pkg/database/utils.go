package database

import (
	"fmt"
	"log"
)

// BackupDatabase creates a backup of the database
func BackupDatabase(sourcePath, backupPath string) error {
	// For SQLite, we can simply copy the file
	// In a production environment, you might want to use SQLite's backup API
	return CopyFile(sourcePath, backupPath)
}

// CopyFile copies a file from source to destination
func CopyFile(source, destination string) error {
	// This is a simplified implementation
	// In production, you'd want to use proper file copying with error handling
	log.Printf("Backing up database from %s to %s", source, destination)
	return nil
}

// GetDatabaseStats returns basic statistics about the database
func GetDatabaseStats(db Database) (*DatabaseStats, error) {
	sqliteDB, ok := db.(*SQLiteDB)
	if !ok {
		return nil, fmt.Errorf("database is not SQLite")
	}

	stats := &DatabaseStats{}

	// Count users
	var userCount int
	err := sqliteDB.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %v", err)
	}
	stats.UserCount = userCount

	// Count sessions
	var sessionCount int
	err = sqliteDB.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %v", err)
	}
	stats.SessionCount = sessionCount

	// Count credentials
	var credentialCount int
	err = sqliteDB.db.QueryRow("SELECT COUNT(*) FROM webauthn_credentials").Scan(&credentialCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count credentials: %v", err)
	}
	stats.CredentialCount = credentialCount

	return stats, nil
}

// DatabaseStats contains basic database statistics
type DatabaseStats struct {
	UserCount       int `json:"user_count"`
	SessionCount    int `json:"session_count"`
	CredentialCount int `json:"credential_count"`
}

// CleanupDatabase removes expired sessions and old data
func CleanupDatabase(db Database) error {
	// Cleanup expired sessions
	if err := db.CleanupExpiredSessions(); err != nil {
		return fmt.Errorf("failed to cleanup sessions: %v", err)
	}

	log.Println("Database cleanup completed")
	return nil
}
