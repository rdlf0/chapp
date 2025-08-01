package auth

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

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

// AuthenticateUser handles the complete authentication flow
func AuthenticateUser() (string, error) {
	fmt.Println("=== CHAPP CLI AUTHENTICATION ===")
	fmt.Println("Opening browser for secure passkey authentication...")

	// Open browser to login page with CLI parameter
	authURL := "http://localhost:8080/login?cli=true"
	if err := openBrowser(authURL); err != nil {
		return "", fmt.Errorf("failed to open browser: %v", err)
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
		return "", fmt.Errorf("no username provided")
	}

	fmt.Printf("Authenticated as: %s\n", username)
	return username, nil
}
