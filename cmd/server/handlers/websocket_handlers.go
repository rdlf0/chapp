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
	// Check for session cookie (web client only)
	cookie, err := r.Cookie(pkgtypes.SessionCookieName)
	if err != nil || cookie.Value == "" {
		log.Printf("WebSocket connection rejected: no session")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Web client with session
	session := auth.GetSession(cookie.Value)
	if session == nil {
		log.Printf("WebSocket connection rejected: invalid session")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	username := session.Username
	log.Printf("Web client connecting: %s", username)

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

	// Send user info to client
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
		log.Printf("Web client connected: %s (registered)", username)
	} else {
		log.Printf("Web client connected: %s (guest)", username)
	}

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump(hub)
}
