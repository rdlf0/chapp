package handlers

import (
	"net/http"
	"os"
	"strings"

	"chapp/cmd/server/auth"
	pkgtypes "chapp/pkg/types"
)

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
	cookie, err := r.Cookie(pkgtypes.SessionCookieName)
	if err != nil || cookie.Value == "" {
		// Redirect to login page if no valid session
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get session
	session := auth.GetSession(cookie.Value)
	if session == nil {
		// Clear invalid cookie and redirect to login
		http.SetCookie(w, &http.Cookie{
			Name:     pkgtypes.SessionCookieName,
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
