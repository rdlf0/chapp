package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// GenerateKeyPair creates a new RSA key pair
func GenerateKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %v", err)
	}
	publicKey := &privateKey.PublicKey
	return privateKey, publicKey, nil
}

// ExportPublicKey exports the public key as base64 string (SPKI format for web compatibility)
func ExportPublicKey(publicKey *rsa.PublicKey) (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pubKeyBytes), nil
}

// ImportPublicKey imports a public key from base64 string (SPKI format for web compatibility)
func ImportPublicKey(keyStr string) (*rsa.PublicKey, error) {
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
