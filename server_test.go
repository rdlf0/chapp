package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestServeHome tests the home page serving
func TestServeHome(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ServeHome)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Chapp") {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}

// TestServeStatic tests static file serving
func TestServeStatic(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"CSS file", "/styles.css", "text/css"},
		{"JS file", "/script.js", "application/javascript"},
		{"Invalid file", "/invalid.js", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(ServeStatic)

			handler.ServeHTTP(rr, req)

			if tt.expected != "" {
				if status := rr.Code; status != http.StatusOK {
					t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
				}
				if contentType := rr.Header().Get("Content-Type"); contentType != tt.expected {
					t.Errorf("handler returned wrong content type: got %v want %v", contentType, tt.expected)
				}
			} else {
				if status := rr.Code; status != http.StatusNotFound {
					t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
				}
			}
		})
	}
}

// TestHub tests hub functionality
func TestHub(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Test client registration
	client := &Client{
		conn:     nil, // Will be set by WebSocket upgrade
		send:     make(chan []byte, 256),
		username: "testuser",
	}

	hub.register <- client
	time.Sleep(10 * time.Millisecond) // Allow goroutine to process

	hub.mutex.RLock()
	clientCount := len(hub.clients)
	hub.mutex.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Test client unregistration
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond) // Allow goroutine to process

	hub.mutex.RLock()
	clientCount = len(hub.clients)
	hub.mutex.RUnlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 clients, got %d", clientCount)
	}
}

// TestMessageBroadcasting tests message broadcasting
func TestMessageBroadcasting(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create test message
	testMsg := Message{
		Type:      "test",
		Content:   "test message",
		Sender:    "testuser",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(testMsg)
	hub.broadcast <- msgBytes

	// Give time for processing
	time.Sleep(10 * time.Millisecond)
}

// TestMessageTypes tests different message type handling
func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name     string
		msgType  string
		content  string
		sender   string
		expected bool
	}{
		{"System message", "system", "User joined", "System", true},
		{"Public key share", "public_key_share", "key123", "user1", true},
		{"Request keys", "request_keys", "", "user1", true},
		{"Encrypted message", "encrypted_message", "encrypted123", "user1", true},
		{"Regular message", "message", "hello", "user1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Type:      tt.msgType,
				Content:   tt.content,
				Sender:    tt.sender,
				Timestamp: time.Now().Unix(),
			}

			msgBytes, err := json.Marshal(msg)
			if err != nil {
				t.Errorf("Failed to marshal message: %v", err)
			}

			// Test that message can be unmarshaled back
			var unmarshaledMsg Message
			err = json.Unmarshal(msgBytes, &unmarshaledMsg)
			if err != nil {
				t.Errorf("Failed to unmarshal message: %v", err)
			}

			if unmarshaledMsg.Type != tt.msgType {
				t.Errorf("Expected message type %s, got %s", tt.msgType, unmarshaledMsg.Type)
			}
		})
	}
}

// TestWebSocketUpgrade tests WebSocket upgrade functionality
func TestWebSocketUpgrade(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=testuser"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Test sending a message
	testMsg := Message{
		Type:      "test",
		Content:   "test message",
		Sender:    "testuser",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(testMsg)
	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)
}

// TestClientReadPump tests client message reading
func TestClientReadPump(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=testuser"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Send a message and verify it's processed
	testMsg := Message{
		Type:      "public_key_share",
		Content:   "testkey",
		Sender:    "testuser",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(testMsg)
	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)
}

// TestConcurrentConnections tests multiple concurrent connections
func TestConcurrentConnections(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	// Connect multiple clients
	clients := make([]*websocket.Conn, 5)
	for i := 0; i < 5; i++ {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=user" + string(rune('0'+i))
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer conn.Close()
		clients[i] = conn
	}

	// Give time for all connections to be established
	time.Sleep(200 * time.Millisecond)

	// Verify all clients are registered
	hub.mutex.RLock()
	clientCount := len(hub.clients)
	hub.mutex.RUnlock()

	if clientCount != 5 {
		t.Errorf("Expected 5 clients, got %d", clientCount)
	}
}

// TestMessageBroadcastingToMultipleClients tests broadcasting to multiple clients
func TestMessageBroadcastingToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	// Connect two clients
	wsURL1 := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=user1"
	wsURL2 := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=user2"

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	// Give time for connections to be established
	time.Sleep(100 * time.Millisecond)

	// Send a message from client 1
	testMsg := Message{
		Type:      "message",
		Content:   "hello from user1",
		Sender:    "user1",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(testMsg)
	err = conn1.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	// Give time for message to be broadcasted
	time.Sleep(100 * time.Millisecond)
}

// TestInvalidMessageHandling tests handling of invalid messages
func TestInvalidMessageHandling(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?username=testuser"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
	if err != nil {
		t.Errorf("Failed to send invalid message: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)
}
