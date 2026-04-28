package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/creack/pty"
)

// AsciicastHeader is the first line of an asciicast v2 file.
type AsciicastHeader struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Env       map[string]string `json:"env,omitempty"`
}

// AsciicastEvent is a single event in the asciicast v2 event stream.
// Serialized as a 3-element JSON array: [time, code, data].
type AsciicastEvent struct {
	Time float64
	Code string
	Data string
}

func (e AsciicastEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{e.Time, e.Code, e.Data})
}

func cmdRecord(args []string) {
	outputFile := "recording.cast"
	if len(args) > 0 {
		outputFile = args[0]
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Get current terminal size
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		width = 80
		height = 24
	}

	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Write header
	header := AsciicastHeader{
		Version:   2,
		Width:     width,
		Height:    height,
		Timestamp: time.Now().Unix(),
		Env: map[string]string{
			"SHELL": shell,
			"TERM":  os.Getenv("TERM"),
		},
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding header: %v\n", err)
		os.Exit(1)
	}
	f.Write(headerJSON)
	f.Write([]byte("\n"))

	// Start PTY session
	cmd := exec.Command(shell)
	cmd.Env = os.Environ()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting PTY: %v\n", err)
		os.Exit(1)
	}
	defer ptmx.Close()

	// Put stdin into raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Handle SIGWINCH (terminal resize)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			w, h, err := term.GetSize(int(os.Stdin.Fd()))
			if err == nil {
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
			}
		}
	}()
	defer signal.Stop(sigCh)

	startTime := time.Now()
	var mu sync.Mutex

	writeEvent := func(code string, data []byte) {
		elapsed := time.Since(startTime).Seconds()
		event := AsciicastEvent{
			Time: elapsed,
			Code: code,
			Data: string(data),
		}
		eventJSON, err := json.Marshal(event)
		if err != nil {
			return
		}
		mu.Lock()
		f.Write(eventJSON)
		f.Write([]byte("\n"))
		mu.Unlock()
	}

	fmt.Fprintf(os.Stderr, "\r\nRecording started. Press Ctrl+D or type 'exit' to stop.\r\n\r\n")

	// Copy stdin -> PTY (user input)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				ptmx.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Copy PTY -> stdout (terminal output) and record events
	buf := make([]byte, 32768)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			writeEvent("o", buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				// PTY closed — normal exit
			}
			break
		}
	}

	// Wait for shell to finish
	cmd.Wait()

	// Restore terminal before printing
	term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Fprintf(os.Stderr, "\nRecording saved to %s\n", outputFile)
}
