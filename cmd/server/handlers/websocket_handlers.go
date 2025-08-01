package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"chapp/cmd/server/auth"
	"chapp/cmd/server/types"
	pkgtypes "chapp/pkg/types"
)

// ServeWs handles WebSocket requests from clients
func ServeWs(hub *types.Hub, w http.ResponseWriter, r *http.Request) {
	var username string

	// Check for session cookie first (web client)
	cookie, err := r.Cookie(pkgtypes.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// Web client with session
		session := auth.GetSession(cookie.Value)
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
	user := auth.GetUser(username)
	if user == nil || !user.IsRegistered {
		log.Printf("WebSocket connection rejected: user not registered with passkey")
		http.Error(w, "Unauthorized - User must be registered with passkey", http.StatusUnauthorized)
		return
	}

	conn, err := types.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &types.Client{
		BaseClient: pkgtypes.BaseClient{
			Conn:     conn,
			Username: username,
		},
		Send: make(chan []byte, 256),
	}

	hub.Register <- client

	// Send user info to client (only for web clients)
	if cookie != nil && cookie.Value != "" {
		// Check if user is registered
		user := auth.GetUser(username)
		isRegistered := user != nil && user.IsRegistered

		userInfoMsg := pkgtypes.Message{
			Type:      pkgtypes.MessageTypeUserInfo,
			Content:   username,
			Sender:    pkgtypes.SystemSender,
			Timestamp: time.Now().Unix(),
		}
		userInfoBytes, _ := json.Marshal(userInfoMsg)
		client.Send <- userInfoBytes

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
