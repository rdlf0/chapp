package utils

import (
	"testing"
)

// TestSplitStringBasic tests basic string splitting functionality
func TestSplitStringBasic(t *testing.T) {
	// Test short string
	result := SplitString("Hello", 10)
	if len(result) != 1 || result[0] != "Hello" {
		t.Errorf("Expected ['Hello'], got %v", result)
	}

	// Test empty string
	result = SplitString("", 10)
	if len(result) != 1 || result[0] != "" {
		t.Errorf("Expected [''], got %v", result)
	}
}
