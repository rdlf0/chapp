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
	"os/exec"
	"runtime"
	"strings"
	"time"

	"chapp/pkg/types"

	"github.com/chzyer/readline"
	"github.com/gorilla/websocket"
)

// AuthCallback represents the authentication callback data
type AuthCallback struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
}

// waitForProtocolCallback waits for authentication and returns the username
func waitForProtocolCallback() string {
	// Poll for temporary auth files
	for i := 0; i < 60; i++ { // 60 seconds timeout
		// Check both /tmp and system temp directory
		tempDirs := []string{"/tmp", os.TempDir()}

		for _, tempDir := range tempDirs {
			files, err := os.ReadDir(tempDir)
			if err != nil {
				continue
			}

			for _, file := range files {
				if strings.HasPrefix(file.Name(), "chapp_auth_") {
					username := strings.TrimPrefix(file.Name(), "chapp_auth_")

					// Read the file to confirm
					filePath := tempDir + "/" + file.Name()
					content, err := os.ReadFile(filePath)
					if err != nil {
						continue
					}

					fileUsername := strings.TrimSpace(string(content))
					if fileUsername == username {
						// Clean up the file
						os.Remove(filePath)
						return username
					}
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	return ""
}

// Client represents the E2E client
type Client struct {
	types.BaseClient
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	otherClients  map[string]string // username -> publicKey
	processedMsgs map[string]bool   // Track processed messages to avoid duplicates
	justSharedKey bool              // Track if we just shared our key recently
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

// EncryptMessage encrypts a message for a specific recipient
func (c *Client) EncryptMessage(message string, recipientPublicKey *rsa.PublicKey) (string, error) {
	// RSA-2048 can encrypt up to ~190 bytes, so we need to chunk longer messages
	const maxChunkSize = 180 // Conservative size to account for padding

	if len(message) <= maxChunkSize {
		// Message is short enough to encrypt directly
		encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, recipientPublicKey, []byte(message), nil)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt message: %v", err)
		}
		return base64.StdEncoding.EncodeToString(encrypted), nil
	}

	// Message is too long, split into chunks
	chunks := splitMessageIntoChunks(message, maxChunkSize)
	var encryptedChunks []string

	for i, chunk := range chunks {
		encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, recipientPublicKey, []byte(chunk), nil)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt chunk %d: %v", i, err)
		}
		encryptedChunks = append(encryptedChunks, base64.StdEncoding.EncodeToString(encrypted))
	}

	// Join encrypted chunks with a separator
	return strings.Join(encryptedChunks, "|"), nil
}

// DecryptMessage decrypts a message using our private key
func (c *Client) DecryptMessage(encryptedMessage string) (string, error) {
	// Check if this is a chunked message (contains separator)
	if strings.Contains(encryptedMessage, "|") {
		// Handle chunked message
		chunks := strings.Split(encryptedMessage, "|")
		var decryptedChunks []string

		for i, chunk := range chunks {
			encryptedBytes, err := base64.StdEncoding.DecodeString(chunk)
			if err != nil {
				return "", fmt.Errorf("failed to decode encrypted chunk %d: %v", i, err)
			}

			decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, encryptedBytes, nil)
			if err != nil {
				return "", fmt.Errorf("failed to decrypt chunk %d: %v", i, err)
			}

			decryptedChunks = append(decryptedChunks, string(decrypted))
		}

		return strings.Join(decryptedChunks, ""), nil
	}

	// Handle single chunk message (original behavior)
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

	message := types.Message{
		Type:      types.MessageTypePublicKeyShare,
		Content:   publicKeyStr,
		Sender:    c.Username,
		Timestamp: time.Now().Unix(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal public key message: %v", err)
	}

	if c.Conn == nil {
		return fmt.Errorf("no WebSocket connection available")
	}

	err = c.Conn.WriteMessage(websocket.TextMessage, messageBytes)
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
		// No other clients, just display locally
		fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), c.Username, content)
		return nil
	}

	// Send encrypted message to each client (except ourselves)
	for recipientUsername, recipientPublicKeyStr := range c.otherClients {
		// Skip sending to ourselves
		if recipientUsername == c.Username {
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

		message := types.Message{
			Type:      types.MessageTypeEncrypted,
			Content:   encryptedContent,
			Sender:    c.Username,
			Recipient: recipientUsername,
			Timestamp: time.Now().Unix(),
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal encrypted message: %v", err)
		}

		if c.Conn == nil {
			return fmt.Errorf("no WebSocket connection available")
		}
		err = c.Conn.WriteMessage(websocket.TextMessage, messageBytes)
		if err != nil {
			return fmt.Errorf("failed to send encrypted message: %v", err)
		}
	}

	// Display our own message locally (don't send to server)
	fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), c.Username, content)

	return nil
}

// Refactored HandleMessage to return a formatted string for display
func (c *Client) HandleMessage(messageBytes []byte) (string, error) {
	var msg types.Message
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
	case types.MessageTypeSystem:
		return fmt.Sprintf("[%s] [SYSTEM] %s", timeString, msg.Content), nil
	case types.MessageTypePublicKeyShare:
		if msg.Sender != c.Username {
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
	case types.MessageTypeEncrypted:
		if msg.Sender != c.Username {
			if msg.Recipient != c.Username {
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
	case types.MessageTypeRequestKeys:
		// Another client is requesting our public key
		if msg.Sender != c.Username {
			go func() {
				time.Sleep(100 * time.Millisecond)
				_ = c.SharePublicKey()
			}()
		}
		return "", nil

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
	fmt.Println("=== Connected Users ===")

	// Add current user first
	fmt.Printf("• %s (you)\n", c.Username)

	// Get sorted list of other users
	otherUsers := make([]string, 0, len(c.otherClients))
	for username := range c.otherClients {
		if username != c.Username {
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
	fmt.Println("=====================")
}

// ShowHelp displays available slash commands
func (c *Client) ShowHelp() {
	fmt.Println("=== Available Commands ===")
	fmt.Println("• /h, /help     - Show this help message")
	fmt.Println("• /q, /quit     - Exit the client")
	fmt.Println("• /list users   - Show all connected users")
	fmt.Println("=======================")
}

// splitString splits a string into lines of specified length
func splitString(s string, maxLength int) []string {
	if len(s) <= maxLength {
		return []string{s}
	}

	var lines []string
	for i := 0; i < len(s); i += maxLength {
		end := i + maxLength
		if end > len(s) {
			end = len(s)
		}
		lines = append(lines, s[i:end])
	}
	return lines
}

// splitMessageIntoChunks splits a message into chunks of specified size
func splitMessageIntoChunks(message string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(message); i += chunkSize {
		end := i + chunkSize
		if end > len(message) {
			end = len(message)
		}
		chunks = append(chunks, message[i:end])
	}
	return chunks
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	// Check if we're running in WSL2
	if runtime.GOOS == "linux" {
		// Try to detect WSL2 by checking for Windows paths or WSL environment
		if _, err := os.Stat("/mnt/c"); err == nil || os.Getenv("WSL_DISTRO_NAME") != "" {
			// We're in WSL2, try multiple methods to open Windows browser

			// Method 1: Try wslview (if available)
			cmd = exec.Command("wslview", url)
			if err := cmd.Start(); err == nil {
				return nil
			}

			// Method 2: Try wsl.exe with start command
			cmd = exec.Command("wsl.exe", "cmd", "/c", "start", url)
			if err := cmd.Start(); err == nil {
				return nil
			}

			// Method 3: Try explorer.exe directly
			cmd = exec.Command("explorer.exe", url)
			if err := cmd.Start(); err == nil {
				return nil
			}

			// If all methods fail, provide helpful error message
			return fmt.Errorf("failed to open browser in WSL2. Please install wslview or run manually: explorer.exe %s", url)
		}
		// Regular Linux, use xdg-open
		cmd = exec.Command("xdg-open", url)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	} else {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

func main() {
	fmt.Println("=== CHAPP CLI AUTHENTICATION ===")
	fmt.Println("Opening browser for secure passkey authentication...")

	// Open browser to login page with CLI parameter
	authURL := "http://localhost:8080/login?cli=true"
	if err := openBrowser(authURL); err != nil {
		log.Fatal("Failed to open browser:", err)
	}

	fmt.Println("Browser opened! Please complete authentication in your browser.")
	fmt.Println("After successful login, you'll be redirected back to the CLI.")
	fmt.Println()

	// Wait for user to complete authentication
	fmt.Println("Waiting for authentication...")
	fmt.Println("(You can also manually enter your username below if needed)")
	fmt.Println()

	// Try to get username from custom protocol (if supported)
	username := waitForProtocolCallback()
	if username == "" {
		// Fallback to manual input
		fmt.Print("Enter your username (from browser authentication): ")
		fmt.Scanln(&username)
	}

	if username == "" {
		log.Fatal("No username provided")
	}

	fmt.Printf("Authenticated as: %s\n", username)
	fmt.Println("Connecting to Chapp server...")

	// Create client
	client := &Client{
		BaseClient: types.BaseClient{
			Username: username,
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair
	if err := client.GenerateKeyPair(); err != nil {
		log.Fatal("Failed to generate key pair:", err)
	}

	// Connect to WebSocket server
	url := fmt.Sprintf("ws://localhost:8080/ws?username=%s", username)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Failed to connect to Chapp server:", err)
	}
	defer conn.Close()
	client.Conn = conn

	// Get the public key for display
	publicKeyStr, err := client.ExportPublicKey()
	if err != nil {
		log.Printf("Warning: Failed to export public key: %v", err)
		publicKeyStr = "ERROR"
	}

	// Create a simple greeting
	fmt.Println("=== CHAPP - E2E ENCRYPTED CHAT ===")
	fmt.Printf("Connected as: %s\n", username)
	fmt.Println("Your Public Key:")
	keyLines := splitString(publicKeyStr, 60)
	for _, line := range keyLines {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println("Type your message or /h for help")
	fmt.Println("==================================")

	// Share public key
	if err := client.SharePublicKey(); err != nil {
		log.Printf("Warning: Failed to share public key: %v", err)
	}

	// Request existing clients to share their keys
	go func() {
		time.Sleep(500 * time.Millisecond) // Small delay to ensure connection is stable
		requestMsg := types.Message{
			Type:      types.MessageTypeRequestKeys,
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
						// Check if it's a server shutdown (close 1006 or 1000)
						if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
							fmt.Println("\nServer disconnected. Exiting...")
						} else {
							log.Printf("Error reading message: %v", err)
						}
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

	// Wait for the message handling goroutine to finish
	<-done
	fmt.Println("Disconnected from Chapp server")
}
