package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type uploadURLRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type uploadURLResponse struct {
	UploadURL   string `json:"upload_url"`
	DownloadURL string `json:"download_url"`
	FileKey     string `json:"file_key"`
	FileID      string `json:"file_id"`
}

type createNoteRequest struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type createNoteResponse struct {
	Slug         string `json:"slug"`
	URLRaw       string `json:"url_raw"`
	URLMarkdown  string `json:"url_markdown"`
	URLRecording string `json:"url_recording,omitempty"`
}

func cmdUpload(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: piece upload <file> [--title TITLE] [--create-note]\n")
		os.Exit(1)
	}

	castFile := args[0]
	title := ""
	createNoteFlag := false

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		if (args[i] == "--title" || args[i] == "-t") && i+1 < len(args) {
			title = args[i+1]
			i++
		}
		if args[i] == "--create-note" {
			createNoteFlag = true
		}
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if cfg.ServerURL == "" {
		fmt.Fprintf(os.Stderr, "Server URL not configured. Run: piece config <server_url>\n")
		os.Exit(1)
	}
	if cfg.Token == "" {
		fmt.Fprintf(os.Stderr, "Not authenticated. Run: piece login\n")
		os.Exit(1)
	}

	// Read the cast file
	fileData, err := os.ReadFile(castFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	filename := filepath.Base(castFile)

	// Step 1: Request presigned upload URL
	fmt.Printf("Requesting upload URL...\n")
	uploadResp, err := requestUploadURL(cfg, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting upload URL: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Upload file to S3 via presigned URL
	fmt.Printf("Uploading %s (%d bytes)...\n", filename, len(fileData))
	if err := uploadToS3(uploadResp.UploadURL, fileData); err != nil {
		fmt.Fprintf(os.Stderr, "Error uploading file: %v\n", err)
		os.Exit(1)
	}

	// For asciicast recordings, return the recording URL directly without creating a note
	if isAsciicastFile(filename) {
		recordingURL := cfg.ServerURL + "/rec/" + uploadResp.FileID + "/p"

		if createNoteFlag {
			fmt.Printf("Creating note...\n")
			if title == "" {
				title = strings.TrimSuffix(filename, filepath.Ext(filename))
			}
			recContent := "<rec id={" + uploadResp.FileID + "} />"
			noteResp, err := createNote(cfg, title, recContent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating note: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\nUpload complete!\n")
			fmt.Printf("Note URL:      %s\n", noteResp.URLMarkdown)
			fmt.Printf("Recording URL: %s\n", recordingURL)
			return
		}

		fmt.Printf("\nUpload complete!\n")
		fmt.Printf("Recording URL: %s\n", recordingURL)
		return
	}

	// Step 3: Create note with the download URL as content (non-recording files)
	fmt.Printf("Creating note...\n")
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	noteResp, err := createNote(cfg, title, uploadResp.DownloadURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating note: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nUpload complete!\n")
	fmt.Printf("Note URL:      %s\n", noteResp.URLMarkdown)
	if noteResp.URLRecording != "" {
		fmt.Printf("Recording URL: %s\n", noteResp.URLRecording)
	}
}

func isAsciicastFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".cast")
}

func requestUploadURL(cfg *Config, filename string) (*uploadURLResponse, error) {
	contentType := "application/octet-stream"
	if isAsciicastFile(filename) {
		contentType = "application/x-asciicast"
	}

	reqBody := uploadURLRequest{
		Filename:    filename,
		ContentType: contentType,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.ServerURL+"/api/files/upload-url", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("%s", errResp.Message)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var urlResp uploadURLResponse
	if err := json.Unmarshal(respBody, &urlResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &urlResp, nil
}

func uploadToS3(presignedURL string, data []byte) error {
	req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-asciicast")
	req.ContentLength = int64(len(data))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func createNote(cfg *Config, title string, downloadURL string) (*createNoteResponse, error) {
	content := downloadURL
	reqBody := createNoteRequest{
		Title:       title,
		Content:     content,
		ContentType: "text/plain",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.ServerURL+"/api/notes", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp errorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("%s", errResp.Message)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var noteResp createNoteResponse
	if err := json.Unmarshal(respBody, &noteResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &noteResp, nil
}
