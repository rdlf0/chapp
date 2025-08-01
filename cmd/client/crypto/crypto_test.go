package crypto

import (
	"strings"
	"testing"
)

// TestGenerateKeyPair tests RSA key pair generation
func TestGenerateKeyPair(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	if privateKey == nil {
		t.Error("Private key should not be nil")
	}

	if publicKey == nil {
		t.Error("Public key should not be nil")
	}

	// Test that public key is the public part of private key
	if publicKey.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Error("Public key should match private key's public part")
	}
}

// TestExportPublicKey tests public key export functionality
func TestExportPublicKey(t *testing.T) {
	_, publicKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	exportedKey, err := ExportPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to export public key: %v", err)
	}

	if exportedKey == "" {
		t.Error("Exported key should not be empty")
	}

	// Test that exported key can be imported back
	importedKey, err := ImportPublicKey(exportedKey)
	if err != nil {
		t.Fatalf("Failed to import public key: %v", err)
	}

	if importedKey == nil {
		t.Fatal("Imported key should not be nil")
	}

	// Test that imported key matches original
	if importedKey.N.Cmp(publicKey.N) != 0 {
		t.Error("Imported key should match original public key")
	}
}

// TestEncryptionDecryption tests message encryption and decryption
func TestEncryptionDecryption(t *testing.T) {
	// Generate key pairs
	privateKey1, publicKey1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client1: %v", err)
	}

	privateKey2, publicKey2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair for client2: %v", err)
	}

	// Test message
	originalMessage := "Hello, this is a test message!"

	// Encrypt message from client1 to client2
	encryptedMessage, err := EncryptMessage(originalMessage, publicKey2)
	if err != nil {
		t.Fatalf("Failed to encrypt message: %v", err)
	}

	if encryptedMessage == "" {
		t.Error("Encrypted message should not be empty")
	}

	// Decrypt message using client2's private key
	decryptedMessage, err := DecryptMessage(encryptedMessage, privateKey2)
	if err != nil {
		t.Fatalf("Failed to decrypt message: %v", err)
	}

	if decryptedMessage != originalMessage {
		t.Errorf("Decrypted message doesn't match original. Got: %s, Expected: %s", decryptedMessage, originalMessage)
	}

	// Test reverse direction
	encryptedMessage2, err := EncryptMessage(originalMessage, publicKey1)
	if err != nil {
		t.Fatalf("Failed to encrypt message in reverse direction: %v", err)
	}

	decryptedMessage2, err := DecryptMessage(encryptedMessage2, privateKey1)
	if err != nil {
		t.Fatalf("Failed to decrypt message in reverse direction: %v", err)
	}

	if decryptedMessage2 != originalMessage {
		t.Errorf("Decrypted message doesn't match original in reverse direction. Got: %s, Expected: %s", decryptedMessage2, originalMessage)
	}
}

// TestLongMessageEncryption tests encryption of long messages that need chunking
func TestLongMessageEncryption(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a long message that will need chunking
	longMessage := strings.Repeat("This is a very long message that will need to be split into chunks. ", 50)

	// Encrypt the long message
	encryptedMessage, err := EncryptMessage(longMessage, publicKey)
	if err != nil {
		t.Fatalf("Failed to encrypt long message: %v", err)
	}

	// Decrypt the long message
	decryptedMessage, err := DecryptMessage(encryptedMessage, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt long message: %v", err)
	}

	if decryptedMessage != longMessage {
		t.Error("Decrypted long message doesn't match original")
	}
}

// TestInvalidKeyImport tests handling of invalid public keys
func TestInvalidKeyImport(t *testing.T) {
	// Test with invalid base64
	_, err := ImportPublicKey("invalid-base64-key")
	if err == nil {
		t.Error("Should fail to import invalid base64 key")
	}

	// Test with empty string
	_, err = ImportPublicKey("")
	if err == nil {
		t.Error("Should fail to import empty key")
	}
}
