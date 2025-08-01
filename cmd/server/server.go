package main

import (
	"log"
	"net/http"

	"chapp/cmd/server/auth"
	"chapp/cmd/server/handlers"
	"chapp/cmd/server/types"
)

func main() {
	// Initialize WebAuthn
	auth.InitializeWebAuthn()

	hub := types.NewHub()
	go hub.Run()

	// Start session cleanup goroutine
	auth.StartSessionCleanup()

	http.HandleFunc("/", handlers.ServeHome)
	http.HandleFunc("/login", handlers.ServeLogin)
	http.HandleFunc("/register", handlers.ServeRegister)
	http.HandleFunc("/logout", handlers.ServeLogout)
	http.HandleFunc("/cli-auth", handlers.ServeCLIAuth) // Add the new CLI auth handler

	// WebAuthn endpoints
	http.HandleFunc("/webauthn/begin-registration", handlers.ServeWebAuthnBeginRegistration)
	http.HandleFunc("/webauthn/finish-registration", handlers.ServeWebAuthnFinishRegistration)
	http.HandleFunc("/webauthn/begin-login", handlers.ServeWebAuthnBeginLogin)
	http.HandleFunc("/webauthn/finish-login", handlers.ServeWebAuthnFinishLogin)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(hub, w, r)
	})

	// Handle static files
	http.HandleFunc("/css/", handlers.ServeStatic)
	http.HandleFunc("/js/", handlers.ServeStatic)

	log.Println("Chapp server starting on :8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
