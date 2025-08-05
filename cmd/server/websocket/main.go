package main

import (
	"log"
	"net/http"

	"chapp/cmd/server/auth"
	"chapp/cmd/server/handlers"
	"chapp/cmd/server/types"
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

	// Initialize WebAuthn (needed for session validation)
	auth.InitializeWebAuthn()

	// Start session cleanup goroutine
	auth.StartSessionCleanup()

	// Create and start hub
	hub := types.NewHub()
	go hub.Run()

	// WebSocket server routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(hub, w, r)
	})

	log.Println("Chapp WebSocket server starting on :8081")

	err = http.ListenAndServe(":8081", mux)
	if err != nil {
		log.Fatal("WebSocket server error: ", err)
	}
}
