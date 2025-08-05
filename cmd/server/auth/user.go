package auth

import (
	"fmt"
	"log"
	"time"

	"chapp/cmd/server/types"
	"chapp/pkg/database"
)

// registerUser creates a new user account
func registerUser(username, passkeyID string) error {
	types.UsersMutex.Lock()
	defer types.UsersMutex.Unlock()

	if _, exists := types.Users[username]; exists {
		return fmt.Errorf("user %s already exists", username)
	}

	user := &types.User{
		Username:     username,
		Created:      time.Now(),
		LastLogin:    time.Now(),
		PasskeyID:    passkeyID,
		IsRegistered: true,
	}

	types.Users[username] = user
	log.Printf("Registered new user: %s", username)
	return nil
}

// GetUser retrieves a user by username
func GetUser(username string) *types.User {
	// Try database first
	db := database.GetDatabase()
	if db != nil {
		user, err := db.GetUser(username)
		if err != nil {
			log.Printf("Failed to get user from database: %v", err)
		} else if user != nil {
			// Convert database user to types.User
			return &types.User{
				Username:     user.Username,
				Created:      user.Created,
				LastLogin:    user.LastLogin,
				PasskeyID:    user.PasskeyID,
				PublicKey:    user.PublicKey,
				IsRegistered: user.IsRegistered,
			}
		}
	}

	// Fallback to memory
	types.UsersMutex.RLock()
	defer types.UsersMutex.RUnlock()
	return types.Users[username]
}

// UpdateUserLastLogin updates the user's last login time
func UpdateUserLastLogin(username string) {
	// Update in database
	db := database.GetDatabase()
	if db != nil {
		if err := db.UpdateUserLastLogin(username); err != nil {
			log.Printf("Failed to update user last login in database: %v", err)
		}
	}

	// Also update in memory
	types.UsersMutex.Lock()
	defer types.UsersMutex.Unlock()

	if user, exists := types.Users[username]; exists {
		user.LastLogin = time.Now()
	}
}

// ValidateUser checks if a user exists and is registered
func ValidateUser(username string) bool {
	user := GetUser(username)
	return user != nil && user.IsRegistered
}

// CreateUserForRegistration creates a new user for WebAuthn registration
func CreateUserForRegistration(username string) *types.User {
	// Create in database
	db := database.GetDatabase()
	if db != nil {
		user, err := db.CreateUser(username)
		if err != nil {
			log.Printf("Failed to create user in database: %v", err)
		} else if user != nil {
			// Convert database user to types.User
			return &types.User{
				Username:     user.Username,
				Created:      user.Created,
				LastLogin:    user.LastLogin,
				PasskeyID:    user.PasskeyID,
				PublicKey:    user.PublicKey,
				IsRegistered: user.IsRegistered,
			}
		}
	}

	// Fallback to memory
	user := &types.User{
		Username:     username,
		Created:      time.Now(),
		LastLogin:    time.Now(),
		IsRegistered: false, // Will be set to true after successful registration
	}

	types.UsersMutex.Lock()
	types.Users[username] = user
	types.UsersMutex.Unlock()

	return user
}

// FindUserByPasskeyID finds a user by their passkey ID
func FindUserByPasskeyID(passkeyID string) *types.User {
	// Try database first
	db := database.GetDatabase()
	if db != nil {
		user, err := db.FindUserByPasskeyID(passkeyID)
		if err != nil {
			log.Printf("Failed to find user by passkey ID in database: %v", err)
		} else if user != nil {
			// Convert database user to types.User
			return &types.User{
				Username:     user.Username,
				Created:      user.Created,
				LastLogin:    user.LastLogin,
				PasskeyID:    user.PasskeyID,
				PublicKey:    user.PublicKey,
				IsRegistered: user.IsRegistered,
			}
		}
	}

	// Fallback to memory
	types.UsersMutex.RLock()
	defer types.UsersMutex.RUnlock()

	for _, user := range types.Users {
		if user.PasskeyID == passkeyID {
			return user
		}
	}
	return nil
}
