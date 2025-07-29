package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStaticFileServing tests that static files are served correctly
func TestStaticFileServing(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedCode int
		contentType  string
	}{
		{"HTML file", "/", http.StatusOK, "text/html"},
		{"CSS file", "/styles.css", http.StatusOK, "text/css"},
		{"JS file", "/script.js", http.StatusOK, "application/javascript"},
		{"Invalid file", "/nonexistent.js", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()

			if tt.path == "/" {
				ServeHome(rr, req)
			} else {
				ServeStatic(rr, req)
			}

			if status := rr.Code; status != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedCode)
			}

			if tt.contentType != "" {
				if contentType := rr.Header().Get("Content-Type"); contentType != tt.contentType {
					t.Errorf("handler returned wrong content type: got %v want %v", contentType, tt.contentType)
				}
			}
		})
	}
}

// TestHTMLContent tests that the HTML contains expected elements
func TestHTMLContent(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	ServeHome(rr, req)

	body := rr.Body.String()

	// Check for essential HTML elements
	expectedElements := []string{
		"<title>Chapp - E2E Chat</title>",
		"<h1>Chapp</h1>",
		"id=\"messages\"",
		"id=\"messageInput\"",
		"id=\"sendButton\"",
		"id=\"clientsList\"",
		"href=\"styles.css\"",
		"src=\"script.js\"",
	}

	for _, element := range expectedElements {
		if !strings.Contains(body, element) {
			t.Errorf("HTML should contain %s", element)
		}
	}
}

// TestHTTPMethods tests that only GET requests are allowed
func TestHTTPMethods(t *testing.T) {
	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			ServeHome(rr, req)

			if status := rr.Code; status != http.StatusMethodNotAllowed {
				t.Errorf("handler returned wrong status code for %s: got %v want %v", method, status, http.StatusMethodNotAllowed)
			}
		})
	}
}

// TestInvalidPaths tests that invalid paths return 404
func TestInvalidPaths(t *testing.T) {
	invalidPaths := []string{"/invalid", "/api/test", "/admin"}

	for _, path := range invalidPaths {
		t.Run(path, func(t *testing.T) {
			req, err := http.NewRequest("GET", path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			ServeHome(rr, req)

			if status := rr.Code; status != http.StatusNotFound {
				t.Errorf("handler returned wrong status code for %s: got %v want %v", path, status, http.StatusNotFound)
			}
		})
	}
}

// TestStaticFileContentTypes tests that static files have correct content types
func TestStaticFileContentTypes(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
	}{
		{"/styles.css", "text/css"},
		{"/script.js", "application/javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			ServeStatic(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			if contentType := rr.Header().Get("Content-Type"); contentType != tt.contentType {
				t.Errorf("handler returned wrong content type: got %v want %v", contentType, tt.contentType)
			}
		})
	}
}

// TestStaticFileNotFound tests that non-existent files return 404
func TestStaticFileNotFound(t *testing.T) {
	invalidFiles := []string{"/nonexistent.css", "/invalid.js", "/test.txt"}

	for _, file := range invalidFiles {
		t.Run(file, func(t *testing.T) {
			req, err := http.NewRequest("GET", file, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			ServeStatic(rr, req)

			if status := rr.Code; status != http.StatusNotFound {
				t.Errorf("handler returned wrong status code for %s: got %v want %v", file, status, http.StatusNotFound)
			}
		})
	}
}

// TestWebSocketEndpoint tests that the WebSocket endpoint is accessible
func TestWebSocketEndpoint(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			ServeWs(hub, w, r)
		} else {
			ServeHome(w, r)
		}
	}))
	defer server.Close()

	// Test that WebSocket endpoint exists
	req, err := http.NewRequest("GET", "/ws", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rr, req)

	// WebSocket upgrade should happen, but we can't easily test it with httptest
	// This test just ensures the endpoint doesn't return 404
	if rr.Code == http.StatusNotFound {
		t.Error("WebSocket endpoint should not return 404")
	}
}

// TestServerStartup tests that the server can start without errors
func TestServerStartup(t *testing.T) {
	// This test verifies that the server can be initialized without errors
	hub := NewHub()
	if hub == nil {
		t.Error("NewHub() should return a non-nil hub")
	}

	if hub.clients == nil {
		t.Error("Hub clients map should be initialized")
	}

	if hub.broadcast == nil {
		t.Error("Hub broadcast channel should be initialized")
	}

	if hub.register == nil {
		t.Error("Hub register channel should be initialized")
	}

	if hub.unregister == nil {
		t.Error("Hub unregister channel should be initialized")
	}
}

// TestMessageStructure tests that Message struct can be marshaled/unmarshaled
func TestMessageStructure(t *testing.T) {
	msg := Message{
		Type:      "test",
		Content:   "test content",
		Sender:    "testuser",
		Recipient: "otheruser",
		Timestamp: 1234567890,
	}

	// Test marshaling
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Test unmarshaling
	var unmarshaledMsg Message
	err = json.Unmarshal(data, &unmarshaledMsg)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	// Verify fields
	if unmarshaledMsg.Type != msg.Type {
		t.Errorf("Type mismatch: got %s, want %s", unmarshaledMsg.Type, msg.Type)
	}

	if unmarshaledMsg.Content != msg.Content {
		t.Errorf("Content mismatch: got %s, want %s", unmarshaledMsg.Content, msg.Content)
	}

	if unmarshaledMsg.Sender != msg.Sender {
		t.Errorf("Sender mismatch: got %s, want %s", unmarshaledMsg.Sender, msg.Sender)
	}

	if unmarshaledMsg.Recipient != msg.Recipient {
		t.Errorf("Recipient mismatch: got %s, want %s", unmarshaledMsg.Recipient, msg.Recipient)
	}

	if unmarshaledMsg.Timestamp != msg.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", unmarshaledMsg.Timestamp, msg.Timestamp)
	}
}
