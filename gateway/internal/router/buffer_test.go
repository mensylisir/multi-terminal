package router

import (
	"io"
	"testing"
)

func TestRingBufferWriteRead(t *testing.T) {
	rb := NewRingBuffer()

	data := []byte("test data")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write length: got %d, want %d", n, len(data))
	}

	// Read should work after write using TryRead
	buf := make([]byte, 1024)

	n, err = rb.TryRead(buf)
	if err != nil {
		t.Fatalf("TryRead failed: %v", err)
	}
	if string(buf[:n]) != "test data" {
		t.Errorf("Read data mismatch: got %q", string(buf[:n]))
	}
}

func TestRingBufferWatermarks(t *testing.T) {
	rb := NewRingBuffer()

	if rb.IsHigh() {
		t.Errorf("New buffer should not be high")
	}
	if !rb.IsLow() {
		t.Errorf("New buffer should be low")
	}
}

func TestRingBufferAvailable(t *testing.T) {
	rb := NewRingBuffer()

	if rb.Available() != 0 {
		t.Errorf("New buffer should have 0 available")
	}

	data := []byte("test")
	rb.Write(data)

	if rb.Available() != len(data) {
		t.Errorf("Available should be %d, got %d", len(data), rb.Available())
	}
}

func TestRingBufferClose(t *testing.T) {
	rb := NewRingBuffer()
	rb.Close()

	// Writing to closed buffer should return EOF
	_, err := rb.Write([]byte("test"))
	if err != io.EOF {
		t.Errorf("Expected EOF on closed buffer, got %v", err)
	}
}
