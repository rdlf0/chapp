package auth

import (
	"log"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthn configuration
var webAuthn *webauthn.WebAuthn

// initializeWebAuthn sets up the WebAuthn configuration
func initializeWebAuthn() {
	config := &webauthn.Config{
		RPDisplayName: "Chapp",
		RPID:          "localhost",                       // Change this for production
		RPOrigins:     []string{"http://localhost:8080"}, // Change this for production
	}

	var err error
	webAuthn, err = webauthn.New(config)
	if err != nil {
		log.Fatal("Failed to initialize WebAuthn:", err)
	}
}

// GetWebAuthn returns the WebAuthn instance
func GetWebAuthn() *webauthn.WebAuthn {
	return webAuthn
}

// InitializeWebAuthn initializes the WebAuthn configuration (exported function)
func InitializeWebAuthn() {
	initializeWebAuthn()
}
