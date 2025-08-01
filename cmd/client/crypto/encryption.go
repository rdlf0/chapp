package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// EncryptMessage encrypts a message for a specific recipient
func EncryptMessage(message string, recipientPublicKey *rsa.PublicKey) (string, error) {
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
func DecryptMessage(encryptedMessage string, privateKey *rsa.PrivateKey) (string, error) {
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

			decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedBytes, nil)
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

	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %v", err)
	}

	return string(decrypted), nil
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
