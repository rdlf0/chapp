package database

import (
	"sync"
)

var (
	dbInstance Database
	once       sync.Once
	mu         sync.RWMutex
)

// SetDatabase sets the global database instance
func SetDatabase(db Database) {
	mu.Lock()
	defer mu.Unlock()
	dbInstance = db
}

// GetDatabase returns the global database instance
func GetDatabase() Database {
	mu.RLock()
	defer mu.RUnlock()
	return dbInstance
}

// CloseDatabase closes the global database instance
func CloseDatabase() error {
	mu.Lock()
	defer mu.Unlock()
	if dbInstance != nil {
		return dbInstance.Close()
	}
	return nil
}
