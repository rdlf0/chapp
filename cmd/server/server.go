package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"chapp/pkg/types"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client
type Client struct {
	types.BaseClient
	send chan []byte
}

// Hub manages all connected clients (server doesn't store private keys)
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Session management
type Session struct {
	Username string
	Created  time.Time
}

var sessions = make(map[string]*Session)
var sessionMutex sync.RWMutex

// generateSessionID creates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// createSession creates a new session for a username
func createSession(username string) string {
	sessionID := generateSessionID()
	session := &Session{
		Username: username,
		Created:  time.Now(),
	}

	sessionMutex.Lock()
	sessions[sessionID] = session
	sessionMutex.Unlock()

	return sessionID
}

// getSession retrieves a session by ID
func getSession(sessionID string) *Session {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	return sessions[sessionID]
}

// deleteSession removes a session
func deleteSession(sessionID string) {
	sessionMutex.Lock()
	delete(sessions, sessionID)
	sessionMutex.Unlock()
}

// cleanupSessions removes old sessions (older than 24 hours)
func cleanupSessions() {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, session := range sessions {
		if session.Created.Before(cutoff) {
			delete(sessions, id)
		}
	}
}

// User management
type User struct {
	Username     string    `json:"username"`
	Created      time.Time `json:"created"`
	LastLogin    time.Time `json:"last_login"`
	PasskeyID    string    `json:"passkey_id,omitempty"`
	PublicKey    string    `json:"public_key,omitempty"`
	IsRegistered bool      `json:"is_registered"`
}

var users = make(map[string]*User)
var usersMutex sync.RWMutex

// registerUser creates a new user account
func registerUser(username, passkeyID string) error {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if _, exists := users[username]; exists {
		return fmt.Errorf("user %s already exists", username)
	}

	user := &User{
		Username:     username,
		Created:      time.Now(),
		LastLogin:    time.Now(),
		PasskeyID:    passkeyID,
		IsRegistered: true,
	}

	users[username] = user
	log.Printf("Registered new user: %s", username)
	return nil
}

// getUser retrieves a user by username
func getUser(username string) *User {
	usersMutex.RLock()
	defer usersMutex.RUnlock()
	return users[username]
}

// updateUserLastLogin updates the user's last login time
func updateUserLastLogin(username string) {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	if user, exists := users[username]; exists {
		user.LastLogin = time.Now()
	}
}

// validateUser checks if a user exists and is registered
func validateUser(username string) bool {
	user := getUser(username)
	return user != nil && user.IsRegistered
}

// WebAuthn configuration
var webAuthn *webauthn.WebAuthn

// WebAuthnUser implements the webauthn.User interface
type WebAuthnUser struct {
	*User
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.Username)
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.Username
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.Username
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	// For now, we'll store credentials in the User struct
	// In a real implementation, you'd have a separate credentials table
	return []webauthn.Credential{}
}

// initializeWebAuthn sets up the WebAuthn configuration
func initializeWebAuthn() {
	config := &webauthn.Config{
		RPDisplayName: "Chapp",
		RPID:          "localhost",                       // Change this for production
		RPOrigins:     []string{"http://localhost:8080"}, // Change this for production
	}

	var err error
	webAuthn, err = webauthn.New(config)
	if err != nil {
		log.Fatal("Failed to initialize WebAuthn:", err)
	}
}

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 100),
		register:   make(chan *Client, 10),
		unregister: make(chan *Client, 10),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

			// Send welcome message (unencrypted system message)
			welcomeMsg := types.Message{
				Type:      types.MessageTypeSystem,
				Content:   fmt.Sprintf("User %s joined the chat", client.Username),
				Sender:    types.SystemSender,
				Timestamp: time.Now().Unix(),
			}
			welcomeBytes, _ := json.Marshal(welcomeMsg)
			h.broadcast <- welcomeBytes

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()

			// Send leave message (unencrypted system message)
			leaveMsg := types.Message{
				Type:      types.MessageTypeSystem,
				Content:   fmt.Sprintf("User %s left the chat", client.Username),
				Sender:    types.SystemSender,
				Timestamp: time.Now().Unix(),
			}
			leaveBytes, _ := json.Marshal(leaveMsg)
			h.broadcast <- leaveBytes

		case message := <-h.broadcast:
			// Parse the message to get sender information
			var msg types.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error parsing broadcast message: %v", err)
				continue
			}

			h.mutex.Lock()
			clientsToRemove := []*Client{}
			for client := range h.clients {
				// Don't send encrypted messages back to the sender
				if msg.Type == types.MessageTypeEncrypted && client.Username == msg.Sender {
					continue
				}

				select {
				case client.send <- message:
					// Message sent successfully
				default:
					close(client.send)
					clientsToRemove = append(clientsToRemove, client)
				}
			}
			// Remove failed clients
			for _, client := range clientsToRemove {
				delete(h.clients, client)
			}
			h.mutex.Unlock()
		}
	}
}

// ReadPump handles reading messages from the WebSocket connection
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Message received (server cannot read encrypted content)

		// Parse the message (server can see metadata but not content)
		var msg types.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Set the sender if not already set
		if msg.Sender == "" {
			msg.Sender = c.Username
		}

		// Set timestamp if not already set
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().Unix()
		}

		// Handle different message types
		switch msg.Type {
		case types.MessageTypeKeyExchange:
			// Handle key exchange - broadcast public key to all clients
			messageBytes, _ := json.Marshal(msg)
			hub.broadcast <- messageBytes

		case types.MessageTypeEncrypted:
			// Handle encrypted message - server cannot decrypt
			messageBytes, _ := json.Marshal(msg)
			hub.broadcast <- messageBytes

		case types.MessageTypePublicKeyShare:
			// Handle public key sharing
			messageBytes, _ := json.Marshal(msg)
			hub.broadcast <- messageBytes

		case types.MessageTypeRequestKeys:
			// Handle key request - broadcast to all clients
			messageBytes, _ := json.Marshal(msg)
			hub.broadcast <- messageBytes

		default:
			// Handle regular message
			messageBytes, _ := json.Marshal(msg)
			hub.broadcast <- messageBytes
		}
	}
}

// WritePump handles writing messages to the WebSocket connection
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for message := range c.send {
		w, err := c.Conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		w.Write(message)

		if err := w.Close(); err != nil {
			return
		}
	}
}

// ServeWs handles WebSocket requests from clients
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var username string

	// Check for session cookie first (web client)
	cookie, err := r.Cookie(types.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// Web client with session
		session := getSession(cookie.Value)
		if session == nil {
			log.Printf("WebSocket connection rejected: invalid session")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		username = session.Username
	} else {
		// CLI client with username parameter (for backward compatibility)
		username = r.URL.Query().Get("username")
		if username == "" {
			log.Printf("WebSocket connection rejected: no session or username")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Check if user is registered with passkey
	user := getUser(username)
	if user == nil || !user.IsRegistered {
		log.Printf("WebSocket connection rejected: user not registered with passkey")
		http.Error(w, "Unauthorized - User must be registered with passkey", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		BaseClient: types.BaseClient{
			Conn:     conn,
			Username: username,
		},
		send: make(chan []byte, 256),
	}

	hub.register <- client

	// Send user info to client (only for web clients)
	if cookie != nil && cookie.Value != "" {
		// Check if user is registered
		user := getUser(username)
		isRegistered := user != nil && user.IsRegistered

		userInfoMsg := types.Message{
			Type:      types.MessageTypeUserInfo,
			Content:   username,
			Sender:    types.SystemSender,
			Timestamp: time.Now().Unix(),
		}
		userInfoBytes, _ := json.Marshal(userInfoMsg)
		client.send <- userInfoBytes

		// Log connection with registration status
		if isRegistered {
			log.Printf("Registered user connected: %s", username)
		} else {
			log.Printf("Guest user connected: %s", username)
		}
	}

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump(hub)
}

// ServeLogin serves the login page and handles form submissions
func ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/login" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		// Serve the passkey-only login page
		paths := []string{"static/login.html", "../static/login.html", "../../static/login.html"}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
		http.Error(w, "Not found", http.StatusNotFound)

	case "POST":
		// Traditional login is no longer supported
		http.Error(w, "Traditional login is not supported. Please use passkey authentication.", http.StatusMethodNotAllowed)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ServeRegister handles user registration requests
func ServeRegister(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/register" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		// Redirect to login page for passkey registration
		http.Redirect(w, r, "/login", http.StatusSeeOther)

	case "POST":
		// Traditional registration is no longer supported
		http.Error(w, "Traditional registration is not supported. Please use passkey registration.", http.StatusMethodNotAllowed)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

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
	if validateUser(username) {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	// Create a new user for WebAuthn registration
	user := &User{
		Username:     username,
		Created:      time.Now(),
		LastLogin:    time.Now(),
		IsRegistered: false, // Will be set to true after successful registration
	}

	webAuthnUser := &WebAuthnUser{User: user}

	// Begin WebAuthn registration
	options, _, err := webAuthn.BeginRegistration(webAuthnUser)
	if err != nil {
		log.Printf("WebAuthn registration failed: %v", err)
		http.Error(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	// The default WebAuthn options should support external authenticators
	// The browser will show all available authenticators including Enpass

	// Store the user temporarily (in production, use a proper session store)
	usersMutex.Lock()
	users[username] = user
	usersMutex.Unlock()

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

	user := getUser(username)
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Create a mock credential for now (in production, you'd validate the actual credential)
	// For now, we'll just mark the user as registered
	usersMutex.Lock()
	user.IsRegistered = true
	user.PasskeyID = req.ID
	usersMutex.Unlock()

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
	var authenticatedUser *User
	usersMutex.RLock()
	for _, user := range users {
		if user.PasskeyID == req.ID {
			authenticatedUser = user
			break
		}
	}
	usersMutex.RUnlock()

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
	updateUserLastLogin(authenticatedUser.Username)

	// Create session and set cookie
	sessionID := createSession(authenticatedUser.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     types.SessionCookieName,
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

// ServeLogout handles logout requests
func ServeLogout(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/logout" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie(types.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// Delete session
		deleteSession(cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     types.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ServeCLIAuth handles CLI authentication redirect
func ServeCLIAuth(w http.ResponseWriter, r *http.Request) {
	// Get session cookie to check if user is authenticated
	cookie, err := r.Cookie(types.SessionCookieName)
	if err != nil || cookie.Value == "" {
		// Not authenticated, redirect to login
		http.Redirect(w, r, "/login?cli=true", http.StatusSeeOther)
		return
	}

	// Get session
	session := getSession(cookie.Value)
	if session == nil {
		// Invalid session, redirect to login
		http.Redirect(w, r, "/login?cli=true", http.StatusSeeOther)
		return
	}

	// Check if user exists and is registered
	user := getUser(session.Username)
	if user == nil || !user.IsRegistered {
		http.Redirect(w, r, "/login?cli=true", http.StatusSeeOther)
		return
	}

	// Write username to temporary file for CLI to read
	tempFile := "/tmp/chapp_auth_" + session.Username
	err = os.WriteFile(tempFile, []byte(session.Username), 0644)
	if err != nil {
		// Try alternative temp directory
		altTempFile := os.TempDir() + "/chapp_auth_" + session.Username
		os.WriteFile(altTempFile, []byte(session.Username), 0644)
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>CLI Authentication Success</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #28a745; font-size: 18px; margin: 20px 0; }
        .info { color: #6c757d; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="success">✅ Authentication Successful!</div>
    <div class="info">Username: <strong>` + session.Username + `</strong></div>
    <div class="info">You can now return to your terminal and use the CLI.</div>
    <div class="info">The CLI should automatically detect your username.</div>
    <script>
        // Auto-close after 3 seconds
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ServeHome serves the HTML page with client-side encryption
func ServeHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for session cookie
	cookie, err := r.Cookie(types.SessionCookieName)
	if err != nil || cookie.Value == "" {
		// Redirect to login page if no valid session
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get session
	session := getSession(cookie.Value)
	if session == nil {
		// Clear invalid cookie and redirect to login
		http.SetCookie(w, &http.Cookie{
			Name:     types.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Try different paths for static files (for both server and test environments)
	paths := []string{"static/index.html", "../static/index.html", "../../static/index.html"}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

// ServeStatic handles static files (CSS, JS) with proper MIME types
func ServeStatic(w http.ResponseWriter, r *http.Request) {
	// Extract the filename from the URL path
	if len(r.URL.Path) <= 1 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	filename := r.URL.Path[1:] // Remove leading slash

	// Set appropriate MIME types based on file extension
	switch {
	case strings.HasSuffix(filename, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(filename, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	default:
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Try different paths for static files (for both server and test environments)
	paths := []string{"static/" + filename, "../static/" + filename, "../../static/" + filename}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

func main() {
	// Initialize WebAuthn
	initializeWebAuthn()

	hub := NewHub()
	go hub.Run()

	// Start session cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanupSessions()
		}
	}()

	http.HandleFunc("/", ServeHome)
	http.HandleFunc("/login", ServeLogin)
	http.HandleFunc("/register", ServeRegister)
	http.HandleFunc("/logout", ServeLogout)
	http.HandleFunc("/cli-auth", ServeCLIAuth) // Add the new CLI auth handler

	// WebAuthn endpoints
	http.HandleFunc("/webauthn/begin-registration", ServeWebAuthnBeginRegistration)
	http.HandleFunc("/webauthn/finish-registration", ServeWebAuthnFinishRegistration)
	http.HandleFunc("/webauthn/begin-login", ServeWebAuthnBeginLogin)
	http.HandleFunc("/webauthn/finish-login", ServeWebAuthnFinishLogin)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	// Handle static files
	http.HandleFunc("/css/", ServeStatic)
	http.HandleFunc("/js/", ServeStatic)

	log.Println("Chapp server starting on :8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
