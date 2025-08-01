package messaging

import (
	"fmt"
	"strings"
)

// CommandHandler handles slash command processing
type CommandHandler struct {
	username     string
	otherClients map[string]string
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(username string, otherClients map[string]string) *CommandHandler {
	return &CommandHandler{
		username:     username,
		otherClients: otherClients,
	}
}

// HandleSlashCommand processes slash commands
func (h *CommandHandler) HandleSlashCommand(command string) (bool, error) {
	command = strings.TrimSpace(command)

	switch {
	case command == "/quit" || command == "/q":
		return true, nil // Signal to quit
	case command == "/h" || command == "/help":
		h.ShowHelp()
		return false, nil // Don't quit, just handled the command
	case strings.HasPrefix(command, "/list users"):
		h.ListUsers()
		return false, nil // Don't quit, just handled the command
	default:
		return false, nil // Not a slash command
	}
}

// ListUsers displays all connected users with their public keys
func (h *CommandHandler) ListUsers() {
	fmt.Println("=== Connected Users ===")

	// Add current user first
	fmt.Printf("• %s (you)\n", h.username)

	// Get sorted list of other users
	otherUsers := make([]string, 0, len(h.otherClients))
	for username := range h.otherClients {
		if username != h.username {
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
		publicKey := h.otherClients[username]
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
func (h *CommandHandler) ShowHelp() {
	fmt.Println("=== Available Commands ===")
	fmt.Println("• /h, /help     - Show this help message")
	fmt.Println("• /q, /quit     - Exit the client")
	fmt.Println("• /list users   - Show all connected users")
	fmt.Println("=======================")
}
