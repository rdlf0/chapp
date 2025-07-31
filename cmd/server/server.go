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
		// CLI client with username parameter (backward compatibility)
		username = r.URL.Query().Get("username")
		if username == "" {
			log.Printf("WebSocket connection rejected: no session or username")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
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
		userInfoMsg := types.Message{
			Type:      types.MessageTypeUserInfo,
			Content:   username,
			Sender:    types.SystemSender,
			Timestamp: time.Now().Unix(),
		}
		userInfoBytes, _ := json.Marshal(userInfoMsg)
		client.send <- userInfoBytes
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
		// Serve the login page
		paths := []string{"static/login.html", "../static/login.html", "../../static/login.html"}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
		http.Error(w, "Not found", http.StatusNotFound)

	case "POST":
		// Handle login form submission
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		if username == "" {
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}

		// Clean username (remove special characters, limit length)
		username = username[:min(len(username), 20)]
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

		// Create session
		sessionID := createSession(username)

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     types.SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   86400, // 24 hours
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
			SameSite: http.SameSiteLaxMode,
		})

		// Redirect to chat
		http.Redirect(w, r, "/", http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
	http.HandleFunc("/logout", ServeLogout)
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
