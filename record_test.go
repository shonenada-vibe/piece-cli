package main

import (
	"encoding/json"
	"testing"
)

func TestAsciicastHeaderMarshal(t *testing.T) {
	header := AsciicastHeader{
		Version:   2,
		Width:     80,
		Height:    24,
		Timestamp: 1504467315,
		Env: map[string]string{
			"SHELL": "/bin/bash",
			"TERM":  "xterm-256color",
		},
	}

	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("failed to marshal header: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal header: %v", err)
	}

	if v := parsed["version"]; v != float64(2) {
		t.Errorf("version = %v, want 2", v)
	}
	if v := parsed["width"]; v != float64(80) {
		t.Errorf("width = %v, want 80", v)
	}
	if v := parsed["height"]; v != float64(24) {
		t.Errorf("height = %v, want 24", v)
	}
	if v := parsed["timestamp"]; v != float64(1504467315) {
		t.Errorf("timestamp = %v, want 1504467315", v)
	}

	env, ok := parsed["env"].(map[string]interface{})
	if !ok {
		t.Fatal("env is not a map")
	}
	if env["SHELL"] != "/bin/bash" {
		t.Errorf("env.SHELL = %v, want /bin/bash", env["SHELL"])
	}
	if env["TERM"] != "xterm-256color" {
		t.Errorf("env.TERM = %v, want xterm-256color", env["TERM"])
	}
}

func TestAsciicastHeaderMinimal(t *testing.T) {
	header := AsciicastHeader{
		Version: 2,
		Width:   80,
		Height:  24,
	}

	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if v := parsed["version"]; v != float64(2) {
		t.Errorf("version = %v, want 2", v)
	}
	// env should be omitted when nil
	if _, ok := parsed["env"]; ok {
		t.Error("env should be omitted when nil")
	}
}

func TestAsciicastEventMarshal(t *testing.T) {
	event := AsciicastEvent{
		Time: 1.001376,
		Code: "o",
		Data: "Hello world",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var parsed []interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("event should be 3 elements, got %d", len(parsed))
	}
	if parsed[0] != 1.001376 {
		t.Errorf("time = %v, want 1.001376", parsed[0])
	}
	if parsed[1] != "o" {
		t.Errorf("code = %v, want 'o'", parsed[1])
	}
	if parsed[2] != "Hello world" {
		t.Errorf("data = %v, want 'Hello world'", parsed[2])
	}
}

func TestAsciicastEventWithEscapeCodes(t *testing.T) {
	event := AsciicastEvent{
		Time: 0.5,
		Code: "o",
		Data: "\033[1;31mHello \033[32mWorld!\033[0m\n",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify it round-trips correctly
	var parsed []interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed[2] != "\033[1;31mHello \033[32mWorld!\033[0m\n" {
		t.Errorf("data did not round-trip correctly: %v", parsed[2])
	}
}

func TestAsciicastEventZeroTime(t *testing.T) {
	event := AsciicastEvent{
		Time: 0,
		Code: "o",
		Data: "first output",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed []interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed[0] != float64(0) {
		t.Errorf("time = %v, want 0", parsed[0])
	}
}

func TestAsciicastFullFile(t *testing.T) {
	// Simulate a minimal asciicast v2 file as newline-delimited JSON
	header := AsciicastHeader{
		Version:   2,
		Width:     80,
		Height:    24,
		Timestamp: 1700000000,
	}
	events := []AsciicastEvent{
		{Time: 0.1, Code: "o", Data: "$ "},
		{Time: 0.5, Code: "o", Data: "ls\r\n"},
		{Time: 1.0, Code: "o", Data: "file1.txt  file2.txt\r\n"},
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}

	// Verify header is a valid JSON object
	var headerMap map[string]interface{}
	if err := json.Unmarshal(headerJSON, &headerMap); err != nil {
		t.Fatalf("header is not valid JSON object: %v", err)
	}

	// Verify each event is a valid JSON array
	for i, ev := range events {
		evJSON, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("event %d marshal failed: %v", i, err)
		}
		var arr []interface{}
		if err := json.Unmarshal(evJSON, &arr); err != nil {
			t.Fatalf("event %d is not valid JSON array: %v", i, err)
		}
		if len(arr) != 3 {
			t.Fatalf("event %d has %d elements, want 3", i, len(arr))
		}
	}
}
