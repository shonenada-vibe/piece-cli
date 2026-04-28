package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"golang.org/x/term"
)

var version = "dev"

func usage() {
	fmt.Fprintf(os.Stderr, `piece %s - https://piece.md CLI

Usage:
  piece config <server_url>    Set the server URL
  piece config                 Show current configuration
  piece login                  Authenticate and store API token
  piece record [output.cast]   Record a terminal session (default: recording.cast)
  piece upload <file.cast>     Upload a recording and create a note
  piece version                Show version information
  piece help                   Show this help message

`, effectiveVersion())
}

func effectiveVersion() string {
	if version != "" && version != "dev" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return version
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		cmdConfig(os.Args[2:])
	case "login":
		cmdLogin(os.Args[2:])
	case "record":
		cmdRecord(os.Args[2:])
	case "upload":
		cmdUpload(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("piece %s\n", effectiveVersion())
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func cmdConfig(args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		// Show current config
		path, _ := configPath()
		fmt.Printf("Config file: %s\n", path)
		fmt.Printf("Server URL:  %s\n", cfg.ServerURL)
		if cfg.Token != "" {
			fmt.Printf("Token:       %s...%s\n", cfg.Token[:4], cfg.Token[len(cfg.Token)-4:])
		} else {
			fmt.Printf("Token:       (not set)\n")
		}
		return
	}

	serverURL := strings.TrimRight(args[0], "/")
	cfg.ServerURL = serverURL
	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Server URL set to: %s\n", serverURL)
}

type loginRequest struct {
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func cmdLogin(args []string) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "Server URL not configured. Run: piece config <server_url>\n")
		os.Exit(1)
	}

	fmt.Print("Username or email: ")
	var identifier string
	fmt.Scanln(&identifier)
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		fmt.Fprintf(os.Stderr, "Username or email is required\n")
		os.Exit(1)
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	password := string(passwordBytes)
	if password == "" {
		fmt.Fprintf(os.Stderr, "Password is required\n")
		os.Exit(1)
	}

	reqBody := loginRequest{Password: password}
	if strings.Contains(identifier, "@") {
		reqBody.Email = identifier
	} else {
		reqBody.Username = identifier
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Post(cfg.ServerURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			fmt.Fprintf(os.Stderr, "Login failed: %s\n", errResp.Message)
		} else {
			fmt.Fprintf(os.Stderr, "Login failed (HTTP %d)\n", resp.StatusCode)
		}
		os.Exit(1)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	cfg.Token = loginResp.AccessToken
	if err := SaveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Login successful! Token expires at %s\n", loginResp.ExpiresAt.Local().Format("2006-01-02 15:04:05"))
}
