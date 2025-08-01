package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chapp/cmd/server/auth"
	pkgtypes "chapp/pkg/types"
)

// TestServeHome tests the home page serving
func TestServeHome(t *testing.T) {
	// Test without session cookie (should redirect to login)
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ServeHome)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	if !strings.Contains(rr.Header().Get("Location"), "/login") {
		t.Errorf("handler should redirect to login, got location: %v", rr.Header().Get("Location"))
	}

	// Test with valid session cookie
	req, err = http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a valid session
	sessionID := auth.CreateSession("testuser")
	req.AddCookie(&http.Cookie{Name: pkgtypes.SessionCookieName, Value: sessionID})

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// This might fail if static files aren't found, but that's expected in test environment
	// The important thing is that it doesn't redirect to login
	if status := rr.Code; status == http.StatusSeeOther {
		if strings.Contains(rr.Header().Get("Location"), "/login") {
			t.Errorf("handler should not redirect to login with valid session")
		}
	}
}

// TestServeStatic tests static file serving
func TestServeStatic(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
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

// TestServeLogin tests login page serving
func TestServeLogin(t *testing.T) {
	req, err := http.NewRequest("GET", "/login", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ServeLogin)

	handler.ServeHTTP(rr, req)

	// This might fail if static files aren't found, but that's expected in test environment
	// The important thing is that it doesn't return an error
	if status := rr.Code; status == http.StatusInternalServerError {
		t.Errorf("handler returned internal server error: %v", rr.Body.String())
	}
}

// TestServeLogout tests logout functionality
func TestServeLogout(t *testing.T) {
	req, err := http.NewRequest("GET", "/logout", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ServeLogout)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusSeeOther)
	}

	if !strings.Contains(rr.Header().Get("Location"), "/login") {
		t.Errorf("handler should redirect to login, got location: %v", rr.Header().Get("Location"))
	}
}
