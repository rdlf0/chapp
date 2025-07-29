package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/gorilla/websocket"
)

// Message represents a chat message
type Message struct {
	Type       string   `json:"type"`
	Content    string   `json:"content"`
	Sender     string   `json:"sender"`
	Recipient  string   `json:"recipient,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Timestamp  int64    `json:"timestamp"`
	Signature  string   `json:"signature,omitempty"`
}

// Client represents the E2E client
type Client struct {
	conn           *websocket.Conn
	username       string
	privateKey     *rsa.PrivateKey
	publicKey      *rsa.PublicKey
	otherClients   map[string]string // username -> publicKey
	hasSharedKey   bool              // Prevent infinite loop
	lastJoinedUser string            // Track last user who joined
	processedMsgs  map[string]bool   // Track processed messages to avoid duplicates
	justSharedKey  bool              // Track if we just shared our key recently
}

// GenerateKeyPair creates a new RSA key pair
func (c *Client) GenerateKeyPair() error {
	var err error
	c.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %v", err)
	}
	c.publicKey = &c.privateKey.PublicKey
	return nil
}

// ExportPublicKey exports the public key as base64 string (SPKI format for web compatibility)
func (c *Client) ExportPublicKey() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(c.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pubKeyBytes), nil
}

// ImportPublicKey imports a public key from base64 string (SPKI format for web compatibility)
func (c *Client) ImportPublicKey(keyStr string) (*rsa.PublicKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	// Try to parse as SPKI format first (web client format)
	pubKey, err := x509.ParsePKIXPublicKey(keyBytes)
	if err != nil {
		// Fallback to PKCS1 format for backward compatibility
		pubKey, err = x509.ParsePKCS1PublicKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key (tried both SPKI and PKCS1): %v", err)
		}
	}

	// Convert to RSA public key
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return rsaPubKey, nil
}

// Helper function to get client keys for debugging
func getClientKeys(clients map[string]string) []string {
	keys := make([]string, 0, len(clients))
	for k := range clients {
		keys = append(keys, k)
	}
	return keys
}

// EncryptMessage encrypts a message for a specific recipient
func (c *Client) EncryptMessage(message string, recipientPublicKey *rsa.PublicKey) (string, error) {
	// Debug: Print recipient key parameters

	encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, recipientPublicKey, []byte(message), nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt message: %v", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptMessage decrypts a message using our private key
func (c *Client) DecryptMessage(encryptedMessage string) (string, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedMessage)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted message: %v", err)
	}

	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, encryptedBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %v", err)
	}

	return string(decrypted), nil
}

// SharePublicKey sends our public key to all clients
func (c *Client) SharePublicKey() error {
	publicKeyStr, err := c.ExportPublicKey()
	if err != nil {
		return err
	}

	message := Message{
		Type:      "public_key_share",
		Content:   publicKeyStr,
		Sender:    c.username,
		Timestamp: time.Now().Unix(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal public key message: %v", err)
	}

	err = c.conn.WriteMessage(websocket.TextMessage, messageBytes)
	if err != nil {
		return fmt.Errorf("failed to send public key: %v", err)
	}

	// Set flag to prevent immediate resharing
	c.justSharedKey = true
	go func() {
		time.Sleep(500 * time.Millisecond) // Reset flag after 500ms
		c.justSharedKey = false
	}()

	return nil
}

// SendEncryptedMessage sends an encrypted message to all other clients
func (c *Client) SendEncryptedMessage(content string) error {
	if len(c.otherClients) == 0 {
		// No other clients, send regular message
		message := Message{
			Type:      "message",
			Content:   content,
			Sender:    c.username,
			Timestamp: time.Now().Unix(),
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %v", err)
		}

		return c.conn.WriteMessage(websocket.TextMessage, messageBytes)
	}

	// Send encrypted message to each client (except ourselves)
	for recipientUsername, recipientPublicKeyStr := range c.otherClients {
		// Skip sending to ourselves
		if recipientUsername == c.username {
			continue
		}

		recipientPublicKey, err := c.ImportPublicKey(recipientPublicKeyStr)
		if err != nil {
			fmt.Printf("Failed to import public key for %s: %v\n", recipientUsername, err)
			continue
		}

		encryptedContent, err := c.EncryptMessage(content, recipientPublicKey)
		if err != nil {
			fmt.Printf("Failed to encrypt message for %s: %v\n", recipientUsername, err)
			continue
		}

		message := Message{
			Type:      "encrypted_message",
			Content:   encryptedContent,
			Sender:    c.username,
			Recipient: recipientUsername,
			Timestamp: time.Now().Unix(),
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal encrypted message: %v", err)
		}

		err = c.conn.WriteMessage(websocket.TextMessage, messageBytes)
		if err != nil {
			return fmt.Errorf("failed to send encrypted message: %v", err)
		}
	}

	// Display our own message locally (don't send to server)
	fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), c.username, content)

	return nil
}

// Refactored HandleMessage to return a formatted string for display
func (c *Client) HandleMessage(messageBytes []byte) (string, error) {
	var msg Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return "", fmt.Errorf("failed to parse message: %v", err)
	}

	// Format timestamp
	var timeString string
	if msg.Timestamp > 0 {
		timestamp := time.Unix(msg.Timestamp, 0)
		timeString = timestamp.Format("15:04:05")
	} else {
		timeString = time.Now().Format("15:04:05")
	}

	switch msg.Type {
	case "system":
		return fmt.Sprintf("[%s] [SYSTEM] %s", timeString, msg.Content), nil
	case "public_key_share":
		if msg.Sender != c.username {
			alreadyHaveKey := c.otherClients[msg.Sender] != ""
			c.otherClients[msg.Sender] = msg.Content
			if !alreadyHaveKey && !c.justSharedKey {
				go func() {
					time.Sleep(100 * time.Millisecond)
					_ = c.SharePublicKey()
				}()
			}
		}
		return "", nil
	case "encrypted_message":
		if msg.Sender != c.username {
			if msg.Recipient != c.username {
				return "", nil
			}
			msgID := fmt.Sprintf("%s_%s_%d", msg.Sender, msg.Content, msg.Timestamp)
			if c.processedMsgs[msgID] {
				return "", nil
			}
			c.processedMsgs[msgID] = true
			decrypted, err := c.DecryptMessage(msg.Content)
			if err != nil {
				return fmt.Sprintf("[%s] [%s] [DECRYPTION FAILED] %s", timeString, msg.Sender, err), nil
			}
			return fmt.Sprintf("[%s] [%s] %s", timeString, msg.Sender, decrypted), nil
		}
		return "", nil
	case "request_keys":
		// Another client is requesting our public key
		if msg.Sender != c.username {
			go func() {
				time.Sleep(100 * time.Millisecond)
				_ = c.SharePublicKey()
			}()
		}
		return "", nil
	case "message":
		return fmt.Sprintf("[%s] [%s] %s", timeString, msg.Sender, msg.Content), nil
	default:
		return fmt.Sprintf("[%s] [%s] %s (type: %s)", timeString, msg.Sender, msg.Content, msg.Type), nil
	}
}

// HandleSlashCommand processes slash commands
func (c *Client) HandleSlashCommand(command string) (bool, error) {
	command = strings.TrimSpace(command)

	switch {
	case command == "/quit" || command == "/q":
		return true, nil // Signal to quit
	case command == "/h" || command == "/help":
		c.ShowHelp()
		return false, nil // Don't quit, just handled the command
	case strings.HasPrefix(command, "/list users"):
		c.ListUsers()
		return false, nil // Don't quit, just handled the command
	default:
		return false, nil // Not a slash command
	}
}

// ListUsers displays all connected users with their public keys
func (c *Client) ListUsers() {
	fmt.Println("\n=== Connected Users ===")

	// Add current user first
	fmt.Printf("• %s (you)\n", c.username)

	// Get sorted list of other users
	otherUsers := make([]string, 0, len(c.otherClients))
	for username := range c.otherClients {
		if username != c.username {
			otherUsers = append(otherUsers, username)
		}
	}

	// Sort alphabetically
	for i := 0; i < len(otherUsers)-1; i++ {
		for j := i + 1; j < len(otherUsers); j++ {
			if otherUsers[i] > otherUsers[j] {
				otherUsers[i], otherUsers[j] = otherUsers[j], otherUsers[i]
			}
		}
	}

	// Display other users
	for _, username := range otherUsers {
		publicKey := c.otherClients[username]
		// Show first 20 characters of public key for readability
		keyPreview := publicKey
		if len(keyPreview) > 20 {
			keyPreview = keyPreview[:20] + "..."
		}
		fmt.Printf("• %s (key: %s)\n", username, keyPreview)
	}

	if len(otherUsers) == 0 {
		fmt.Println("No other users connected")
	}
	fmt.Println("=====================\n")
}

// ShowHelp displays available slash commands
func (c *Client) ShowHelp() {
	fmt.Println("\n=== Available Commands ===")
	fmt.Println("• /h, /help     - Show this help message")
	fmt.Println("• /q, /quit     - Exit the client")
	fmt.Println("• /list users   - Show all connected users")
	fmt.Println("=======================\n")
}

func main() {
	// Get username from command line or prompt
	var username string
	if len(os.Args) > 1 {
		username = os.Args[1]
	} else {
		fmt.Print("Enter your username: ")
		fmt.Scanln(&username)
		if username == "" {
			username = "Anonymous"
		}
	}

	// Create client
	client := &Client{
		username:      username,
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair
	fmt.Println("🔑 Generating RSA key pair...")
	if err := client.GenerateKeyPair(); err != nil {
		log.Fatal("Failed to generate key pair:", err)
	}
	fmt.Println("✅ RSA key pair generated successfully!")

	// Connect to WebSocket server
	url := fmt.Sprintf("ws://localhost:8080/ws?username=%s", username)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Failed to connect to Chapp server:", err)
	}
	defer conn.Close()
	client.conn = conn

	fmt.Printf("✅ Connected to Chapp server as %s\n", username)
	fmt.Println("Type your message or /h for help")

	// Share public key
	if err := client.SharePublicKey(); err != nil {
		log.Printf("Warning: Failed to share public key: %v", err)
	}

	// Request existing clients to share their keys
	go func() {
		time.Sleep(500 * time.Millisecond) // Small delay to ensure connection is stable
		requestMsg := Message{
			Type:      "request_keys",
			Sender:    username,
			Timestamp: time.Now().Unix(),
		}
		requestBytes, _ := json.Marshal(requestMsg)
		conn.WriteMessage(websocket.TextMessage, requestBytes)
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
						log.Println("Error reading message:", err)
					}
					return
				}
				if display, err := client.HandleMessage(message); err == nil && display != "" {
					rl.Write([]byte(display + "\n"))
				}
			}
		}
	}()

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
			shouldQuit, err := client.HandleSlashCommand(text)
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
		if err := client.SendEncryptedMessage(text); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
	<-done
	fmt.Println("\nDisconnected from Chapp server")
}
