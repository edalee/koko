package main

import (
	"testing"
	"time"
)

func TestRingBuffer_Basic(t *testing.T) {
	rb := newRingBuffer(10)
	rb.Write([]byte("hello"))

	got := string(rb.Bytes())
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	rb := newRingBuffer(5)
	rb.Write([]byte("abcdefgh")) // 8 bytes into 5-byte buffer

	got := string(rb.Bytes())
	if got != "defgh" {
		t.Fatalf("expected 'defgh', got %q", got)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := newRingBuffer(10)
	got := rb.Bytes()
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(got))
	}
}

func TestRingBuffer_ExactSize(t *testing.T) {
	rb := newRingBuffer(5)
	rb.Write([]byte("abcde"))

	got := string(rb.Bytes())
	if got != "abcde" {
		t.Fatalf("expected 'abcde', got %q", got)
	}
}

func TestSubscribe_NonexistentSession(t *testing.T) {
	tm := NewTerminalManager()
	ch := tm.Subscribe("nonexistent")
	if ch != nil {
		t.Fatal("expected nil channel for nonexistent session")
	}
}

func TestSubscribe_Unsubscribe(t *testing.T) {
	tm := NewTerminalManager()

	// Manually inject a session (bypasses PTY creation)
	s := &session{
		id:          "test-1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-1"] = s
	tm.mu.Unlock()

	ch := tm.Subscribe("test-1")
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	s.mu.Lock()
	count := len(s.subscribers)
	s.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	tm.Unsubscribe("test-1", ch)

	s.mu.Lock()
	count = len(s.subscribers)
	s.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 subscribers, got %d", count)
	}
}

func TestSubscriberFanOut(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-2",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-2"] = s
	tm.mu.Unlock()

	ch1 := tm.Subscribe("test-2")
	ch2 := tm.Subscribe("test-2")

	// Simulate what readLoop does: fan out data to subscribers
	data := []byte("test output")
	s.mu.Lock()
	for ch := range s.subscribers {
		select {
		case ch <- append([]byte(nil), data...):
		default:
		}
	}
	s.mu.Unlock()

	// Both subscribers should receive the data
	select {
	case got := <-ch1:
		if string(got) != "test output" {
			t.Fatalf("ch1: expected 'test output', got %q", string(got))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1: timeout waiting for data")
	}

	select {
	case got := <-ch2:
		if string(got) != "test output" {
			t.Fatalf("ch2: expected 'test output', got %q", string(got))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2: timeout waiting for data")
	}
}

func TestReadOutput(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-3",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	s.tailText.Write([]byte("some terminal output"))

	tm.mu.Lock()
	tm.sessions["test-3"] = s
	tm.mu.Unlock()

	output, err := tm.ReadOutput("test-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "some terminal output" {
		t.Fatalf("expected 'some terminal output', got %q", output)
	}
}

func TestReadOutput_NotFound(t *testing.T) {
	tm := NewTerminalManager()

	_, err := tm.ReadOutput("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestReadOutput_NilTailText(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-4",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-4"] = s
	tm.mu.Unlock()

	output, err := tm.ReadOutput("test-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty, got %q", output)
	}
}

func TestGetSessionState_NonexistentReturnsIdle(t *testing.T) {
	tm := NewTerminalManager()
	state := tm.GetSessionState("nonexistent")
	if state != "idle" {
		t.Fatalf("expected 'idle', got %q", state)
	}
}
