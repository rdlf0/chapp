package types

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"chapp/pkg/types"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client
type Client struct {
	types.BaseClient
	Send chan []byte
}

// Hub manages all connected clients (server doesn't store private keys)
type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	Mutex      sync.RWMutex
}

// Session management
type Session struct {
	Username string
	Created  time.Time
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

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte, 100),
		Register:   make(chan *Client, 10),
		Unregister: make(chan *Client, 10),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Mutex.Lock()
			h.Clients[client] = true
			h.Mutex.Unlock()

			// Send welcome message (unencrypted system message)
			welcomeMsg := types.Message{
				Type:      types.MessageTypeSystem,
				Content:   fmt.Sprintf("User %s joined the chat", client.Username),
				Sender:    types.SystemSender,
				Timestamp: time.Now().Unix(),
			}
			welcomeBytes, _ := json.Marshal(welcomeMsg)
			h.Broadcast <- welcomeBytes

		case client := <-h.Unregister:
			h.Mutex.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.Mutex.Unlock()

			// Send leave message (unencrypted system message)
			leaveMsg := types.Message{
				Type:      types.MessageTypeSystem,
				Content:   fmt.Sprintf("User %s left the chat", client.Username),
				Sender:    types.SystemSender,
				Timestamp: time.Now().Unix(),
			}
			leaveBytes, _ := json.Marshal(leaveMsg)
			h.Broadcast <- leaveBytes

		case message := <-h.Broadcast:
			// Parse the message to get sender information
			var msg types.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error parsing broadcast message: %v", err)
				continue
			}

			h.Mutex.Lock()
			clientsToRemove := []*Client{}
			for client := range h.Clients {
				// Don't send encrypted messages back to the sender
				if msg.Type == types.MessageTypeEncrypted && client.Username == msg.Sender {
					continue
				}

				select {
				case client.Send <- message:
					// Message sent successfully
				default:
					close(client.Send)
					clientsToRemove = append(clientsToRemove, client)
				}
			}
			// Remove failed clients
			for _, client := range clientsToRemove {
				delete(h.Clients, client)
			}
			h.Mutex.Unlock()
		}
	}
}

// ReadPump handles reading messages from the WebSocket connection
func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.Unregister <- c
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
			hub.Broadcast <- messageBytes

		case types.MessageTypeEncrypted:
			// Handle encrypted message - server cannot decrypt
			messageBytes, _ := json.Marshal(msg)
			hub.Broadcast <- messageBytes

		case types.MessageTypePublicKeyShare:
			// Handle public key sharing
			messageBytes, _ := json.Marshal(msg)
			hub.Broadcast <- messageBytes

		case types.MessageTypeRequestKeys:
			// Handle key request - broadcast to all clients
			messageBytes, _ := json.Marshal(msg)
			hub.Broadcast <- messageBytes

		default:
			// Handle regular message
			messageBytes, _ := json.Marshal(msg)
			hub.Broadcast <- messageBytes
		}
	}
}

// WritePump handles writing messages to the WebSocket connection
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for message := range c.Send {
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

// Global variables (will be moved to appropriate modules)
var (
	Sessions     = make(map[string]*Session)
	SessionMutex sync.RWMutex
	Users        = make(map[string]*User)
	UsersMutex   sync.RWMutex
	Upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}
)
