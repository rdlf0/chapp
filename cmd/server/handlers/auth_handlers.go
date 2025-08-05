package handlers

import (
	"log"
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

// ServeCLIAuth handles CLI authentication redirect
func ServeCLIAuth(w http.ResponseWriter, r *http.Request) {
	// For CLI authentication, we need to get the username from the request
	// This should be passed from the WebAuthn login flow
	username := r.URL.Query().Get("username")
	log.Printf("CLI auth handler - Username parameter: %s", username)

	if username == "" {
		// No username provided, redirect to login
		log.Printf("CLI auth handler - No username provided, redirecting to login")
		http.Redirect(w, r, "/login?cli=true", http.StatusSeeOther)
		return
	}

	// Check if user exists and is registered
	user := auth.GetUser(username)
	if user == nil || !user.IsRegistered {
		http.Redirect(w, r, "/login?cli=true", http.StatusSeeOther)
		return
	}

	// Write username to temporary file for CLI to read
	tempFile := "/tmp/chapp_auth_" + username
	err := os.WriteFile(tempFile, []byte(username), 0644)
	if err != nil {
		// Try alternative temp directory
		altTempFile := os.TempDir() + "/chapp_auth_" + username
		os.WriteFile(altTempFile, []byte(username), 0644)
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>CLI Authentication Success</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #28a745; font-size: 18px; margin: 20px 0; }
        .info { color: #6c757d; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="success">✅ Authentication Successful!</div>
    <div class="info">Username: <strong>` + username + `</strong></div>
    <div class="info">You can now return to your terminal and use the CLI.</div>
    <div class="info">The CLI should automatically detect your username.</div>
    <script>
        // Auto-close after 3 seconds
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}
