package handlers

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"

	"chapp/cmd/server/auth"
	"chapp/cmd/server/types"
)

// ServeWebAuthnBeginRegistration starts the WebAuthn registration process
func ServeWebAuthnBeginRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Clean username (remove special characters, limit length)
	username := req.Username[:min(len(req.Username), 20)]
	for i, char := range username {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-') {
			username = username[:i] + username[i+1:]
		}
	}

	if len(username) < 2 {
		http.Error(w, "Username must be at least 2 characters", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	if auth.ValidateUser(username) {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	// Create a new user for WebAuthn registration
	user := auth.CreateUserForRegistration(username)

	webAuthnUser := &types.WebAuthnUser{User: user}

	// Begin WebAuthn registration
	options, _, err := auth.GetWebAuthn().BeginRegistration(webAuthnUser)
	if err != nil {
		log.Printf("WebAuthn registration failed: %v", err)
		http.Error(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	// Return the registration options
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// ServeWebAuthnFinishRegistration completes the WebAuthn registration process
func ServeWebAuthnFinishRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the credential creation response from the request body
	var req struct {
		ID       string `json:"id"`
		RawID    string `json:"rawId"`
		Type     string `json:"type"`
		Username string `json:"username"` // Add username to the request structure
		Response struct {
			AttestationObject string `json:"attestationObject"`
			ClientDataJSON    string `json:"clientDataJSON"`
		} `json:"response"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to parse credential creation response: %v", err)
		http.Error(w, "Invalid response", http.StatusBadRequest)
		return
	}

	// Get username from the request body
	username := req.Username
	if username == "" {
		http.Error(w, "Username not found in request", http.StatusBadRequest)
		return
	}

	user := auth.GetUser(username)
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Create a mock credential for now (in production, you'd validate the actual credential)
	// For now, we'll just mark the user as registered
	types.UsersMutex.Lock()
	user.IsRegistered = true
	user.PasskeyID = req.ID
	types.UsersMutex.Unlock()

	log.Printf("WebAuthn registration completed for user: %s", username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// ServeWebAuthnBeginLogin starts the WebAuthn authentication process
func ServeWebAuthnBeginLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate a random challenge for WebAuthn
	challenge := make([]byte, 32)
	_, err := rand.Read(challenge)
	if err != nil {
		log.Printf("Failed to generate challenge: %v", err)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	// Return WebAuthn authentication options
	response := map[string]interface{}{
		"publicKey": map[string]interface{}{
			"challenge":        challenge,
			"rpId":             "localhost",
			"timeout":          60000,
			"userVerification": "preferred",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ServeWebAuthnFinishLogin completes the WebAuthn authentication process
func ServeWebAuthnFinishLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the credential assertion response from the request body
	var req struct {
		ID       string `json:"id"`
		RawID    string `json:"rawId"`
		Type     string `json:"type"`
		Response struct {
			AuthenticatorData string `json:"authenticatorData"`
			ClientDataJSON    string `json:"clientDataJSON"`
			Signature         string `json:"signature"`
		} `json:"response"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to parse credential request response: %v", err)
		http.Error(w, "Invalid response", http.StatusBadRequest)
		return
	}

	// Find the user by passkey ID
	authenticatedUser := auth.FindUserByPasskeyID(req.ID)

	if authenticatedUser == nil {
		http.Error(w, "User not found or passkey not recognized", http.StatusNotFound)
		return
	}

	if !authenticatedUser.IsRegistered {
		http.Error(w, "User not fully registered", http.StatusUnauthorized)
		return
	}

	// For now, we'll just mark the user as logged in
	// In production, you'd validate the actual credential
	auth.UpdateUserLastLogin(authenticatedUser.Username)

	// Create session and set cookie
	sessionID := auth.CreateSession(authenticatedUser.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     "chapp_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400, // 24 hours
	})

	log.Printf("WebAuthn login completed for user: %s", authenticatedUser.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"username": authenticatedUser.Username,
	})
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
