package messaging

import (
	"testing"
)

// TestNewCommandHandler tests command handler creation
func TestNewCommandHandler(t *testing.T) {
	otherClients := map[string]string{
		"user1": "key1",
		"user2": "key2",
	}

	handler := NewCommandHandler("testuser", otherClients)

	if handler.username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", handler.username)
	}

	if len(handler.otherClients) != 2 {
		t.Errorf("Expected 2 other clients, got %d", len(handler.otherClients))
	}

	if handler.otherClients["user1"] != "key1" {
		t.Errorf("Expected key1 for user1, got %s", handler.otherClients["user1"])
	}

	if handler.otherClients["user2"] != "key2" {
		t.Errorf("Expected key2 for user2, got %s", handler.otherClients["user2"])
	}
}

// TestHandleSlashCommand tests slash command handling
func TestHandleSlashCommand(t *testing.T) {
	handler := NewCommandHandler("testuser", make(map[string]string))

	// Test help command
	shouldQuit, err := handler.HandleSlashCommand("/help")
	if err != nil {
		t.Fatalf("Failed to handle help command: %v", err)
	}
	if shouldQuit {
		t.Error("Help command should not signal quit")
	}

	// Test quit command
	shouldQuit, err = handler.HandleSlashCommand("/quit")
	if err != nil {
		t.Fatalf("Failed to handle quit command: %v", err)
	}
	if !shouldQuit {
		t.Error("Quit command should signal quit")
	}

	// Test unknown command
	shouldQuit, err = handler.HandleSlashCommand("/unknown")
	if err != nil {
		t.Fatalf("Failed to handle unknown command: %v", err)
	}
	if shouldQuit {
		t.Error("Unknown command should not signal quit")
	}

	// Test empty command
	shouldQuit, err = handler.HandleSlashCommand("/")
	if err != nil {
		t.Fatalf("Failed to handle empty command: %v", err)
	}
	if shouldQuit {
		t.Error("Empty command should not signal quit")
	}
}

// TestListUsers tests user listing functionality
func TestListUsers(t *testing.T) {
	otherClients := map[string]string{
		"user1": "key1",
		"user2": "key2",
		"user3": "key3",
	}
	handler := NewCommandHandler("testuser", otherClients)

	// This test mainly ensures the function doesn't panic
	// In a real test, you might capture stdout to verify output
	handler.ListUsers()
}

// TestShowHelp tests help display functionality
func TestShowHelp(t *testing.T) {
	handler := NewCommandHandler("testuser", make(map[string]string))

	// This test mainly ensures the function doesn't panic
	// In a real test, you might capture stdout to verify output
	handler.ShowHelp()
}

// TestCommandHandlerWithEmptyClients tests command handler with no other clients
func TestCommandHandlerWithEmptyClients(t *testing.T) {
	handler := NewCommandHandler("testuser", make(map[string]string))

	if len(handler.otherClients) != 0 {
		t.Errorf("Expected 0 other clients, got %d", len(handler.otherClients))
	}

	// Test that commands still work with no other clients
	shouldQuit, err := handler.HandleSlashCommand("/help")
	if err != nil {
		t.Fatalf("Failed to handle help command with empty clients: %v", err)
	}
	if shouldQuit {
		t.Error("Help command should not signal quit")
	}
}
