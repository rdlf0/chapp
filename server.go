package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a chat message (server can't read encrypted content)
type Message struct {
	Type       string   `json:"type"`
	Content    string   `json:"content"` // Encrypted by client
	Sender     string   `json:"sender"`
	Recipient  string   `json:"recipient,omitempty"`  // For encrypted messages
	Recipients []string `json:"recipients,omitempty"` // For private messages
	Timestamp  int64    `json:"timestamp"`
	Signature  string   `json:"signature,omitempty"` // Client-generated signature
}

// Client represents a connected WebSocket client
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	username  string
	publicKey string // Client's public key (shared with others)
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

			log.Printf("Client registered: %s (total clients: %d)", client.username, len(h.clients))

			// Send welcome message (unencrypted system message)
			welcomeMsg := Message{
				Type:      "system",
				Content:   fmt.Sprintf("User %s joined the chat", client.username),
				Sender:    "System",
				Timestamp: time.Now().Unix(),
			}
			welcomeBytes, _ := json.Marshal(welcomeMsg)
			select {
			case h.broadcast <- welcomeBytes:
				log.Printf("Welcome message sent for: %s", client.username)
			default:
				log.Printf("Warning: Could not send welcome message for: %s (channel full)", client.username)
			}

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()

			// Send leave message (unencrypted system message)
			leaveMsg := Message{
				Type:      "system",
				Content:   fmt.Sprintf("User %s left the chat", client.username),
				Sender:    "System",
				Timestamp: time.Now().Unix(),
			}
			leaveBytes, _ := json.Marshal(leaveMsg)
			select {
			case h.broadcast <- leaveBytes:
				log.Printf("Leave message sent for: %s", client.username)
			default:
				log.Printf("Warning: Could not send leave message for: %s (channel full)", client.username)
			}

			log.Printf("Client unregistered: %s", client.username)

		case message := <-h.broadcast:
			// Parse the message to get sender information
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error parsing broadcast message: %v", err)
				continue
			}

			h.mutex.Lock()
			clientsToRemove := []*Client{}
			for client := range h.clients {
				// Don't send encrypted messages back to the sender
				if msg.Type == "encrypted_message" && client.username == msg.Sender {
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
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Message received (server cannot read encrypted content)

		// Parse the message (server can see metadata but not content)
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Set the sender if not already set
		if msg.Sender == "" {
			msg.Sender = c.username
		}

		// Set timestamp if not already set
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().Unix()
		}

		// Handle different message types
		switch msg.Type {
		case "key_exchange":
			// Handle key exchange - broadcast public key to all clients
			messageBytes, _ := json.Marshal(msg)
			select {
			case hub.broadcast <- messageBytes:
				// Key exchange broadcasted
			default:
				log.Printf("Warning: Could not broadcast key exchange from %s", msg.Sender)
			}

		case "encrypted_message":
			// Handle encrypted message - server cannot decrypt
			messageBytes, _ := json.Marshal(msg)
			select {
			case hub.broadcast <- messageBytes:
				// Encrypted message broadcasted
			default:
				log.Printf("Warning: Could not broadcast encrypted message from %s", msg.Sender)
			}

		case "public_key_share":
			// Handle public key sharing
			messageBytes, _ := json.Marshal(msg)
			select {
			case hub.broadcast <- messageBytes:
				// Public key broadcasted
			default:
				log.Printf("Warning: Could not broadcast public key from %s", msg.Sender)
			}

		case "request_keys":
			// Handle key request - broadcast to all clients
			messageBytes, _ := json.Marshal(msg)
			select {
			case hub.broadcast <- messageBytes:
				// Key request broadcasted
			default:
				log.Printf("Warning: Could not broadcast key request from %s", msg.Sender)
			}

		default:
			// Handle regular message
			log.Printf("Regular message from %s (content: [ENCRYPTED])", c.username)
			messageBytes, _ := json.Marshal(msg)
			select {
			case hub.broadcast <- messageBytes:
				log.Printf("Message broadcasted from %s", msg.Sender)
			default:
				log.Printf("Warning: Could not broadcast message from %s (channel full)", msg.Sender)
			}
		}
	}
}

// WritePump handles writing messages to the WebSocket connection
func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// ServeWs handles WebSocket requests from clients
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	log.Printf("Connection attempt from %s", r.RemoteAddr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Get username from query parameter
	username := r.URL.Query().Get("username")
	if username == "" {
		username = "Anonymous"
	}

	log.Printf("Connection established for user: %s", username)

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 256),
		username: username,
	}

	hub.register <- client

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump(hub)
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

	// Serve the static HTML file
	http.ServeFile(w, r, "static/index.html")
}

// ServeStatic handles static files (CSS, JS) with proper MIME types
func ServeStatic(w http.ResponseWriter, r *http.Request) {
	// Extract the filename from the URL path
	filename := r.URL.Path[1:] // Remove leading slash

	// Set appropriate MIME types
	switch {
	case filename == "styles.css":
		w.Header().Set("Content-Type", "text/css")
	case filename == "script.js":
		w.Header().Set("Content-Type", "application/javascript")
	default:
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Serve the file
	http.ServeFile(w, r, "static/"+filename)
}

func main() {
	hub := NewHub()
	go hub.Run()

	http.HandleFunc("/", ServeHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	// Handle static files
	http.HandleFunc("/styles.css", ServeStatic)
	http.HandleFunc("/script.js", ServeStatic)

	log.Println("Chapp starting on :8080")
	log.Println("Open http://localhost:8080 in your browser")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
