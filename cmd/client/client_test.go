package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"chapp/pkg/types"
)

// TestClientKeyGeneration tests RSA key pair generation
func TestClientKeyGeneration(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	err := client.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if client.privateKey == nil {
		t.Error("Private key should not be nil")
	}

	if client.publicKey == nil {
		t.Error("Public key should not be nil")
	}

	// Test that public key is the public part of private key
	if client.publicKey.N.Cmp(client.privateKey.PublicKey.N) != 0 {
		t.Error("Public key should match private key's public part")
	}
}

// TestExportPublicKey tests public key export functionality
func TestExportPublicKey(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	err := client.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	exportedKey, err := client.ExportPublicKey()
	if err != nil {
		t.Fatalf("Failed to export public key: %v", err)
	}

	if exportedKey == "" {
		t.Error("Exported key should not be empty")
	}

	// Test that exported key can be imported back
	importedKey, err := client.ImportPublicKey(exportedKey)
	if err != nil {
		t.Fatalf("Failed to import public key: %v", err)
	}

	if importedKey == nil {
		t.Fatal("Imported key should not be nil")
	}

	// Test that imported key matches original
	if importedKey.N.Cmp(client.publicKey.N) != 0 {
		t.Error("Imported key should match original public key")
	}
}

// TestEncryptionDecryption tests message encryption and decryption
func TestEncryptionDecryption(t *testing.T) {
	// Create two clients
	client1 := &Client{
		BaseClient: types.BaseClient{
			Username: "user1",
		},
		otherClients: make(map[string]string),
	}

	client2 := &Client{
		BaseClient: types.BaseClient{
			Username: "user2",
		},
		otherClients: make(map[string]string),
	}

	// Generate key pairs
	err := client1.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client1: %v", err)
	}

	err = client2.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client2: %v", err)
	}

	// Export public keys
	pubKey1, err := client1.ExportPublicKey()
	if err != nil {
		t.Fatalf("Failed to export public key from client1: %v", err)
	}

	_, err = client2.ExportPublicKey()
	if err != nil {
		t.Fatalf("Failed to export public key from client2: %v", err)
	}

	// Import public keys
	importedKey1, err := client2.ImportPublicKey(pubKey1)
	if err != nil {
		t.Fatalf("Failed to import public key in client2: %v", err)
	}

	// Test message encryption and decryption
	testMessage := "Hello, this is a test message!"
	encrypted, err := client2.EncryptMessage(testMessage, importedKey1)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}

	if encrypted == "" {
		t.Error("Encrypted message should not be empty")
	}

	// Decrypt the message
	decrypted, err := client1.DecryptMessage(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	if decrypted != testMessage {
		t.Errorf("Decrypted message doesn't match original. Expected: %s, Got: %s", testMessage, decrypted)
	}
}

// TestHandleSlashCommands tests slash command handling
func TestHandleSlashCommands(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	tests := []struct {
		name       string
		command    string
		shouldQuit bool
	}{
		{"Quit command", "/quit", true},
		{"Short quit command", "/q", true},
		{"Help command", "/h", false},
		{"Long help command", "/help", false},
		{"List users command", "/list users", false},
		{"Invalid command", "/invalid", false},
		{"Not a slash command", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldQuit, err := client.HandleSlashCommand(tt.command)
			if err != nil {
				t.Errorf("HandleSlashCommand failed: %v", err)
			}

			if shouldQuit != tt.shouldQuit {
				t.Errorf("Expected shouldQuit=%v, got %v", tt.shouldQuit, shouldQuit)
			}
		})
	}
}

// TestListUsers tests the ListUsers function
func TestListUsers(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	// Add some test clients
	client.otherClients["alice"] = "key1"
	client.otherClients["bob"] = "key2"
	client.otherClients["charlie"] = "key3"

	// Test that ListUsers doesn't crash
	client.ListUsers()
}

// TestShowHelp tests the ShowHelp function
func TestShowHelp(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	// Test that ShowHelp doesn't crash
	client.ShowHelp()
}

// TestHandleMessage tests message handling
func TestHandleMessage(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients: make(map[string]string),
	}

	// Generate key pair for encryption tests
	err := client.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tests := []struct {
		name         string
		message      types.Message
		expectOutput bool
	}{
		{
			name: "System message",
			message: types.Message{
				Type:      "system",
				Content:   "User joined",
				Sender:    "System",
				Timestamp: time.Now().Unix(),
			},
			expectOutput: true,
		},
		{
			name: "Public key share",
			message: types.Message{
				Type:      "public_key_share",
				Content:   "testkey",
				Sender:    "otheruser",
				Timestamp: time.Now().Unix(),
			},
			expectOutput: false, // Public key shares don't produce output
		},
		{
			name: "Request keys",
			message: types.Message{
				Type:      "request_keys",
				Content:   "",
				Sender:    "otheruser",
				Timestamp: time.Now().Unix(),
			},
			expectOutput: false, // Request keys don't produce output
		},
		{
			name: "Regular message",
			message: types.Message{
				Type:      "message",
				Content:   "Hello",
				Sender:    "otheruser",
				Timestamp: time.Now().Unix(),
			},
			expectOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgBytes, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			output, err := client.HandleMessage(msgBytes)
			if err != nil {
				t.Errorf("HandleMessage failed: %v", err)
			}

			if tt.expectOutput && output == "" {
				t.Error("Expected output but got empty string")
			}

			if !tt.expectOutput && output != "" {
				t.Errorf("Expected no output but got: %s", output)
			}
		})
	}
}

// TestEncryptedMessageHandling tests handling of encrypted messages
func TestEncryptedMessageHandling(t *testing.T) {
	// Create two clients
	client1 := &Client{
		BaseClient: types.BaseClient{
			Username: "user1",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	client2 := &Client{
		BaseClient: types.BaseClient{
			Username: "user2",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pairs
	err := client1.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client1: %v", err)
	}

	err = client2.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client2: %v", err)
	}

	// Exchange public keys
	pubKey1, _ := client1.ExportPublicKey()
	pubKey2, _ := client2.ExportPublicKey()

	client1.otherClients["user2"] = pubKey2
	client2.otherClients["user1"] = pubKey1

	// Create an encrypted message
	testMessage := "Secret message"
	importedKey, _ := client2.ImportPublicKey(pubKey1)
	encrypted, _ := client2.EncryptMessage(testMessage, importedKey)

	// Test handling encrypted message
	encryptedMsg := types.Message{
		Type:      "encrypted_message",
		Content:   encrypted,
		Sender:    "user2",
		Recipient: "user1",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(encryptedMsg)
	output, err := client1.HandleMessage(msgBytes)
	if err != nil {
		t.Fatalf("Failed to handle encrypted message: %v", err)
	}

	if !strings.Contains(output, testMessage) {
		t.Errorf("Expected decrypted message to contain '%s', got: %s", testMessage, output)
	}
}

// TestMessageDeduplication tests message deduplication
func TestMessageDeduplication(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Create a test message
	testMsg := types.Message{
		Type:      "encrypted_message",
		Content:   "testcontent",
		Sender:    "otheruser",
		Recipient: "testuser",
		Timestamp: time.Now().Unix(),
	}

	msgBytes, _ := json.Marshal(testMsg)

	// Process the same message twice
	output1, err1 := client.HandleMessage(msgBytes)
	output2, err2 := client.HandleMessage(msgBytes)

	if err1 != nil {
		t.Errorf("First message handling failed: %v", err1)
	}

	if err2 != nil {
		t.Errorf("Second message handling failed: %v", err2)
	}

	// The second message should be ignored (deduplicated)
	if output1 == output2 && output1 != "" {
		t.Error("Duplicate message should be ignored")
	}
}

// TestKeySharing tests public key sharing functionality
func TestKeySharing(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair
	err := client.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Test SharePublicKey (this would normally require a WebSocket connection)
	// For now, we just test that it doesn't crash
	err = client.SharePublicKey()
	// This will fail because there's no WebSocket connection, but that's expected
	if err == nil {
		t.Error("SharePublicKey should fail without WebSocket connection")
	}
}

// TestSendEncryptedMessage tests sending encrypted messages
func TestSendEncryptedMessage(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair
	err := client.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Test SendEncryptedMessage with no other clients (should succeed and display locally)
	err = client.SendEncryptedMessage("test message")
	if err != nil {
		t.Errorf("SendEncryptedMessage should succeed when no other clients: %v", err)
	}
}

// TestInvalidKeyImport tests handling of invalid public keys
func TestInvalidKeyImport(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Test importing invalid key
	_, err := client.ImportPublicKey("invalid_key")
	if err == nil {
		t.Error("ImportPublicKey should fail with invalid key")
	}
}

// TestMessageTimestampHandling tests message timestamp handling
func TestMessageTimestampHandling(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair for decryption
	if err := client.GenerateKeyPair(); err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	tests := []struct {
		name      string
		timestamp int64
	}{
		{"Current timestamp", time.Now().Unix()},
		{"Zero timestamp", 0},
		{"Future timestamp", time.Now().Unix() + 3600},
		{"Past timestamp", time.Now().Unix() - 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt the message properly
			encryptedContent, err := client.EncryptMessage("test", client.publicKey)
			if err != nil {
				t.Fatalf("Failed to encrypt message: %v", err)
			}

			msg := types.Message{
				Type:      types.MessageTypeEncrypted,
				Content:   encryptedContent,
				Sender:    "otheruser",
				Recipient: "testuser",
				Timestamp: tt.timestamp,
			}

			msgBytes, _ := json.Marshal(msg)
			output, err := client.HandleMessage(msgBytes)
			if err != nil {
				t.Errorf("HandleMessage failed: %v", err)
			}

			if output == "" {
				t.Error("Expected output but got empty string")
			}
		})
	}
}

// TestConcurrentMessageHandling tests concurrent message handling
func TestConcurrentMessageHandling(t *testing.T) {
	client := &Client{
		BaseClient: types.BaseClient{
			Username: "testuser",
		},
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
	}

	// Generate key pair for decryption
	if err := client.GenerateKeyPair(); err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create multiple messages
	messages := make([]types.Message, 10)
	for i := 0; i < 10; i++ {
		// Encrypt each message properly
		encryptedContent, err := client.EncryptMessage(fmt.Sprintf("message %d", i), client.publicKey)
		if err != nil {
			t.Fatalf("Failed to encrypt message %d: %v", i, err)
		}

		messages[i] = types.Message{
			Type:      types.MessageTypeEncrypted,
			Content:   encryptedContent,
			Sender:    "otheruser",
			Recipient: "testuser",
			Timestamp: time.Now().Unix(),
		}
	}

	// Process messages concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(msg types.Message) {
			msgBytes, _ := json.Marshal(msg)
			_, err := client.HandleMessage(msgBytes)
			if err != nil {
				t.Errorf("Concurrent message handling failed: %v", err)
			}
			done <- true
		}(messages[i])
	}

	// Wait for all messages to be processed
	for i := 0; i < 10; i++ {
		<-done
	}
}
