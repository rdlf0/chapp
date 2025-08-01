package messaging

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"time"

	"chapp/cmd/client/crypto"
	"chapp/pkg/types"

	"github.com/gorilla/websocket"
)

// MessageHandler handles message processing and display
type MessageHandler struct {
	username      string
	privateKey    *rsa.PrivateKey
	otherClients  map[string]string // username -> publicKey
	processedMsgs map[string]bool   // Track processed messages to avoid duplicates
	justSharedKey bool              // Track if we just shared our key recently
	conn          *websocket.Conn
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(username string, privateKey *rsa.PrivateKey, conn *websocket.Conn) *MessageHandler {
	return &MessageHandler{
		username:      username,
		privateKey:    privateKey,
		otherClients:  make(map[string]string),
		processedMsgs: make(map[string]bool),
		conn:          conn,
	}
}

// SharePublicKey sends our public key to all clients
func (h *MessageHandler) SharePublicKey() error {
	publicKeyStr, err := crypto.ExportPublicKey(&h.privateKey.PublicKey)
	if err != nil {
		return err
	}

	message := types.Message{
		Type:      types.MessageTypePublicKeyShare,
		Content:   publicKeyStr,
		Sender:    h.username,
		Timestamp: time.Now().Unix(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal public key message: %v", err)
	}

	if h.conn == nil {
		return fmt.Errorf("no WebSocket connection available")
	}

	err = h.conn.WriteMessage(websocket.TextMessage, messageBytes)
	if err != nil {
		return fmt.Errorf("failed to send public key: %v", err)
	}

	// Set flag to prevent immediate resharing
	h.justSharedKey = true
	go func() {
		time.Sleep(500 * time.Millisecond) // Reset flag after 500ms
		h.justSharedKey = false
	}()

	return nil
}

// SendEncryptedMessage sends an encrypted message to all other clients
func (h *MessageHandler) SendEncryptedMessage(content string) error {
	if len(h.otherClients) == 0 {
		// No other clients, just display locally
		fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), h.username, content)
		return nil
	}

	// Send encrypted message to each client (except ourselves)
	for recipientUsername, recipientPublicKeyStr := range h.otherClients {
		// Skip sending to ourselves
		if recipientUsername == h.username {
			continue
		}

		recipientPublicKey, err := crypto.ImportPublicKey(recipientPublicKeyStr)
		if err != nil {
			fmt.Printf("Failed to import public key for %s: %v\n", recipientUsername, err)
			continue
		}

		encryptedContent, err := crypto.EncryptMessage(content, recipientPublicKey)
		if err != nil {
			fmt.Printf("Failed to encrypt message for %s: %v\n", recipientUsername, err)
			continue
		}

		message := types.Message{
			Type:      types.MessageTypeEncrypted,
			Content:   encryptedContent,
			Sender:    h.username,
			Recipient: recipientUsername,
			Timestamp: time.Now().Unix(),
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal encrypted message: %v", err)
		}

		if h.conn == nil {
			return fmt.Errorf("no WebSocket connection available")
		}
		err = h.conn.WriteMessage(websocket.TextMessage, messageBytes)
		if err != nil {
			return fmt.Errorf("failed to send encrypted message: %v", err)
		}
	}

	// Display our own message locally (don't send to server)
	fmt.Printf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), h.username, content)

	return nil
}

// HandleMessage processes incoming messages and returns a formatted string for display
func (h *MessageHandler) HandleMessage(messageBytes []byte) (string, error) {
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
		if msg.Sender != h.username {
			alreadyHaveKey := h.otherClients[msg.Sender] != ""
			h.otherClients[msg.Sender] = msg.Content
			if !alreadyHaveKey && !h.justSharedKey {
				go func() {
					time.Sleep(100 * time.Millisecond)
					_ = h.SharePublicKey()
				}()
			}
		}
		return "", nil
	case types.MessageTypeEncrypted:
		if msg.Sender != h.username {
			if msg.Recipient != h.username {
				return "", nil
			}
			msgID := fmt.Sprintf("%s_%s_%d", msg.Sender, msg.Content, msg.Timestamp)
			if h.processedMsgs[msgID] {
				return "", nil
			}
			h.processedMsgs[msgID] = true
			decrypted, err := crypto.DecryptMessage(msg.Content, h.privateKey)
			if err != nil {
				return fmt.Sprintf("[%s] [%s] [DECRYPTION FAILED] %s", timeString, msg.Sender, err), nil
			}
			return fmt.Sprintf("[%s] [%s] %s", timeString, msg.Sender, decrypted), nil
		}
		return "", nil
	case types.MessageTypeRequestKeys:
		// Another client is requesting our public key
		if msg.Sender != h.username {
			go func() {
				time.Sleep(100 * time.Millisecond)
				_ = h.SharePublicKey()
			}()
		}
		return "", nil

	default:
		return fmt.Sprintf("[%s] [%s] %s (type: %s)", timeString, msg.Sender, msg.Content, msg.Type), nil
	}
}

// RequestKeys requests existing clients to share their keys
func (h *MessageHandler) RequestKeys() error {
	requestMsg := types.Message{
		Type:      types.MessageTypeRequestKeys,
		Sender:    h.username,
		Timestamp: time.Now().Unix(),
	}
	requestBytes, _ := json.Marshal(requestMsg)
	return h.conn.WriteMessage(websocket.TextMessage, requestBytes)
}

// GetOtherClients returns the map of other clients
func (h *MessageHandler) GetOtherClients() map[string]string {
	return h.otherClients
}
