package main

import (
	"log"
	"net/http"

	"chapp/cmd/server/auth"
	"chapp/cmd/server/handlers"
	"chapp/pkg/database"
)

func main() {
	// Initialize database
	db, err := database.NewSQLite("chapp.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Set global database instance
	database.SetDatabase(db)

	// Initialize WebAuthn
	auth.InitializeWebAuthn()

	// Start session cleanup goroutine
	auth.StartSessionCleanup()

	// Static server routes (authentication, pages, static files)
	http.HandleFunc("/", handlers.ServeHome)
	http.HandleFunc("/login", handlers.ServeLogin)
	http.HandleFunc("/register", handlers.ServeRegister)
	http.HandleFunc("/logout", handlers.ServeLogout)

	// WebAuthn endpoints
	http.HandleFunc("/webauthn/begin-registration", handlers.ServeWebAuthnBeginRegistration)
	http.HandleFunc("/webauthn/finish-registration", handlers.ServeWebAuthnFinishRegistration)
	http.HandleFunc("/webauthn/begin-login", handlers.ServeWebAuthnBeginLogin)
	http.HandleFunc("/webauthn/finish-login", handlers.ServeWebAuthnFinishLogin)

	// Handle static files
	http.HandleFunc("/css/", handlers.ServeStatic)
	http.HandleFunc("/js/", handlers.ServeStatic)

	log.Println("Chapp static server starting on :8080")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Static server error: ", err)
	}
}
