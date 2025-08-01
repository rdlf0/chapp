package auth

import (
	"fmt"
	"log"
	"time"

	"chapp/cmd/server/types"
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
	types.UsersMutex.RLock()
	defer types.UsersMutex.RUnlock()
	return types.Users[username]
}

// UpdateUserLastLogin updates the user's last login time
func UpdateUserLastLogin(username string) {
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
	types.UsersMutex.RLock()
	defer types.UsersMutex.RUnlock()

	for _, user := range types.Users {
		if user.PasskeyID == passkeyID {
			return user
		}
	}
	return nil
}
