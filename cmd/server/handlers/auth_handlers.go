package handlers

import (
	"net/http"
	"os"

	"chapp/cmd/server/auth"
	pkgtypes "chapp/pkg/types"
)

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
	cookie, err := r.Cookie(pkgtypes.SessionCookieName)
	if err == nil && cookie.Value != "" {
		// Delete session
		auth.DeleteSession(cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     pkgtypes.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
