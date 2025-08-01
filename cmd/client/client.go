package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"chapp/cmd/client/auth"
	"chapp/cmd/client/crypto"
	"chapp/cmd/client/messaging"
	"chapp/cmd/client/utils"

	"github.com/chzyer/readline"
	"github.com/gorilla/websocket"
)

func main() {
	// Authenticate user
	username, err := auth.AuthenticateUser()
	if err != nil {
		log.Fatal("Authentication failed:", err)
	}

	fmt.Println("Connecting to Chapp server...")

	// Generate key pair
	privateKey, publicKey, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair:", err)
	}

	// Connect to WebSocket server
	url := fmt.Sprintf("ws://localhost:8080/ws?username=%s", username)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Failed to connect to Chapp server:", err)
	}
	defer conn.Close()

	// Get the public key for display
	publicKeyStr, err := crypto.ExportPublicKey(publicKey)
	if err != nil {
		log.Printf("Warning: Failed to export public key: %v", err)
		publicKeyStr = "ERROR"
	}

	// Create a simple greeting
	fmt.Println("=== CHAPP - E2E ENCRYPTED CHAT ===")
	fmt.Printf("Connected as: %s\n", username)
	fmt.Println("Your Public Key:")
	keyLines := utils.SplitString(publicKeyStr, 60)
	for _, line := range keyLines {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println("Type your message or /h for help")
	fmt.Println("==================================")

	// Create message handler
	messageHandler := messaging.NewMessageHandler(username, privateKey, conn)

	// Share public key
	if err := messageHandler.SharePublicKey(); err != nil {
		log.Printf("Warning: Failed to share public key: %v", err)
	}

	// Request existing clients to share their keys
	go func() {
		time.Sleep(500 * time.Millisecond) // Small delay to ensure connection is stable
		messageHandler.RequestKeys()
	}()

	// Replace bufio.Scanner with readline
	rl, err := readline.New("> ")
	if err != nil {
		log.Fatalf("failed to create readline: %v", err)
	}
	defer rl.Close()
	done := make(chan struct{})
	shutdown := make(chan struct{})

	// Goroutine to handle incoming messages
	go func() {
		defer close(done)
		for {
			select {
			case <-shutdown:
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Don't log error if we're shutting down
					select {
					case <-shutdown:
						return
					default:
						// Check if it's a server shutdown (close 1006 or 1000)
						if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
							fmt.Println("\nServer disconnected. Exiting...")
						} else {
							log.Printf("Error reading message: %v", err)
						}
					}
					return
				}
				if display, err := messageHandler.HandleMessage(message); err == nil && display != "" {
					rl.Write([]byte(display + "\n"))
				}
			}
		}
	}()

	// Create command handler
	commandHandler := messaging.NewCommandHandler(username, messageHandler.GetOtherClients())

	// Read user input and send messages
	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, Ctrl-D, etc.
			break
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}

		// Check for slash commands first
		if strings.HasPrefix(text, "/") {
			shouldQuit, err := commandHandler.HandleSlashCommand(text)
			if err != nil {
				log.Printf("Error handling command: %v", err)
			}
			if shouldQuit {
				fmt.Println("Disconnecting...")
				close(shutdown) // Signal the goroutine to stop
				conn.Close()    // Close the connection
				return          // Exit the main function immediately
			}
			continue // Skip sending as regular message
		}

		// Handle legacy "quit" command
		if text == "quit" {
			break
		}

		// Send as regular encrypted message
		if err := messageHandler.SendEncryptedMessage(text); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}

	// Wait for the message handling goroutine to finish
	<-done
	fmt.Println("Disconnected from Chapp server")
}
