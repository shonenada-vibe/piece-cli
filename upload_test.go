package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestUploadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/files/upload-url" {
			t.Errorf("expected /api/files/upload-url, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		var req uploadURLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Filename != "demo.cast" {
			t.Errorf("filename = %q, want demo.cast", req.Filename)
		}
		if req.ContentType != "application/x-asciicast" {
			t.Errorf("content_type = %q, want application/x-asciicast", req.ContentType)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(uploadURLResponse{
			UploadURL:   "https://s3.example.com/presigned-put",
			DownloadURL: "https://cdn.example.com/files/demo.cast",
			FileKey:     "user123/demo.cast",
			FileID:      "file-uuid-123",
		})
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Token: "test-token"}
	resp, err := requestUploadURL(cfg, "demo.cast")
	if err != nil {
		t.Fatalf("requestUploadURL failed: %v", err)
	}
	if resp.UploadURL != "https://s3.example.com/presigned-put" {
		t.Errorf("uploadURL = %q, want presigned URL", resp.UploadURL)
	}
	if resp.DownloadURL != "https://cdn.example.com/files/demo.cast" {
		t.Errorf("downloadURL = %q, want CDN URL", resp.DownloadURL)
	}
	if resp.FileID != "file-uuid-123" {
		t.Errorf("fileID = %q, want file-uuid-123", resp.FileID)
	}
}

func TestRequestUploadURLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "UNAUTHORIZED",
			"message": "authentication required",
		})
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Token: "bad-token"}
	_, err := requestUploadURL(cfg, "demo.cast")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "authentication required" {
		t.Errorf("error = %q, want 'authentication required'", err.Error())
	}
}

func TestUploadToS3(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		receivedContentType = r.Header.Get("Content-Type")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	testData := []byte(`{"version":2,"width":80,"height":24}`)
	err := uploadToS3(server.URL, testData)
	if err != nil {
		t.Fatalf("uploadToS3 failed: %v", err)
	}
	if string(receivedBody) != string(testData) {
		t.Errorf("received body = %q, want %q", receivedBody, testData)
	}
	if receivedContentType != "application/x-asciicast" {
		t.Errorf("content-type = %q, want application/x-asciicast", receivedContentType)
	}
}

func TestUploadToS3Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access Denied"))
	}))
	defer server.Close()

	err := uploadToS3(server.URL, []byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/notes" {
			t.Errorf("expected /api/notes, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		var req createNoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if req.Title != "My Recording" {
			t.Errorf("title = %q, want 'My Recording'", req.Title)
		}
		if req.Content != "https://cdn.example.com/files/demo.cast" {
			t.Errorf("content = %q, want download URL", req.Content)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createNoteResponse{
			Slug:         "abc123",
			URLMarkdown:  "https://example.com/md/abc123",
			URLRecording: "https://example.com/notes/abc123/recording",
		})
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Token: "test-token"}
	resp, err := createNote(cfg, "My Recording", "https://cdn.example.com/files/demo.cast")
	if err != nil {
		t.Fatalf("createNote failed: %v", err)
	}
	if resp.Slug != "abc123" {
		t.Errorf("slug = %q, want abc123", resp.Slug)
	}
	if resp.URLRecording != "https://example.com/notes/abc123/recording" {
		t.Errorf("url_recording = %q, want recording URL", resp.URLRecording)
	}
}

func TestCreateNote_WithRecEmbed(t *testing.T) {
	fileID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	expectedContent := "<rec id={" + fileID + "} />"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createNoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if req.Content != expectedContent {
			t.Errorf("content = %q, want %q", req.Content, expectedContent)
		}
		if req.Title != "my-recording" {
			t.Errorf("title = %q, want 'my-recording'", req.Title)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createNoteResponse{
			Slug:        "slug123",
			URLMarkdown: "https://example.com/md/slug123",
		})
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Token: "test-token"}
	resp, err := createNote(cfg, "my-recording", expectedContent)
	if err != nil {
		t.Fatalf("createNote failed: %v", err)
	}
	if resp.Slug != "slug123" {
		t.Errorf("slug = %q, want slug123", resp.Slug)
	}
}

func TestIsAsciicastFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"demo.cast", true},
		{"DEMO.CAST", true},
		{"recording.Cast", true},
		{"image.png", false},
		{"file.txt", false},
	}
	for _, tt := range tests {
		if got := isAsciicastFile(tt.filename); got != tt.want {
			t.Errorf("isAsciicastFile(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestCreateNoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "CONTENT_REQUIRED",
			"message": "content is required",
		})
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Token: "test-token"}
	_, err := createNote(cfg, "title", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
