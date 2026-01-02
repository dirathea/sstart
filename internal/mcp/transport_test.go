package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestStdioTransport_ReadMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid request",
			input:   `{"jsonrpc":"2.0","id":1,"method":"test/method"}` + "\n",
			wantErr: false,
		},
		{
			name:    "valid response",
			input:   `{"jsonrpc":"2.0","id":1,"result":{"success":true}}` + "\n",
			wantErr: false,
		},
		{
			name:    "with CRLF",
			input:   `{"jsonrpc":"2.0","id":1,"method":"test"}` + "\r\n",
			wantErr: false,
		},
		{
			name:    "empty line then valid",
			input:   "\n" + `{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n",
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}` + "\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			transport := NewStdioTransport(reader, io.Discard)

			msg, err := transport.ReadMessage()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg == nil {
				t.Error("expected message, got nil")
			}
		})
	}
}

func TestStdioTransport_WriteMessage(t *testing.T) {
	var buf bytes.Buffer
	transport := NewStdioTransport(strings.NewReader(""), &buf)

	msg, _ := NewJSONRPCRequest(1, "test/method", nil)
	if err := transport.WriteMessage(msg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	output := buf.String()

	// Should end with newline
	if !strings.HasSuffix(output, "\n") {
		t.Error("expected output to end with newline")
	}

	// Should be valid JSON
	var parsed JSONRPCMessage
	if err := json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.Method != "test/method" {
		t.Errorf("expected method test/method, got %s", parsed.Method)
	}
}

func TestStdioTransport_ReadWriteRoundTrip(t *testing.T) {
	// Create a pipe for testing
	var buf bytes.Buffer

	writeTransport := NewStdioTransport(strings.NewReader(""), &buf)

	// Write a message
	original, _ := NewJSONRPCRequest(123, "tools/list", nil)
	if err := writeTransport.WriteMessage(original); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Read it back
	readTransport := NewStdioTransport(&buf, io.Discard)
	parsed, err := readTransport.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if parsed.Method != original.Method {
		t.Errorf("method mismatch: expected %s, got %s", original.Method, parsed.Method)
	}
}

func TestStdioTransport_Close(t *testing.T) {
	transport := NewStdioTransport(strings.NewReader(""), io.Discard)

	if err := transport.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Subsequent operations should fail
	_, err := transport.ReadMessage()
	if err == nil {
		t.Error("expected error reading from closed transport")
	}

	msg, _ := NewJSONRPCRequest(1, "test", nil)
	err = transport.WriteMessage(msg)
	if err == nil {
		t.Error("expected error writing to closed transport")
	}
}

func TestStdioTransport_ReadMultipleMessages(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"first"}
{"jsonrpc":"2.0","id":2,"method":"second"}
{"jsonrpc":"2.0","id":3,"method":"third"}
`
	transport := NewStdioTransport(strings.NewReader(input), io.Discard)

	expectedMethods := []string{"first", "second", "third"}
	for i, expected := range expectedMethods {
		msg, err := transport.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message %d: %v", i+1, err)
		}
		if msg.Method != expected {
			t.Errorf("message %d: expected method %s, got %s", i+1, expected, msg.Method)
		}
	}

	// Next read should return EOF
	_, err := transport.ReadMessage()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestTrimNewline(t *testing.T) {
	tests := []struct {
		input    []byte
		expected []byte
	}{
		{[]byte("hello\n"), []byte("hello")},
		{[]byte("hello\r\n"), []byte("hello")},
		{[]byte("hello"), []byte("hello")},
		{[]byte("\n"), []byte{}},
		{[]byte("\r\n"), []byte{}},
		{[]byte(""), []byte{}},
	}

	for _, tt := range tests {
		result := trimNewline(tt.input)
		if !bytes.Equal(result, tt.expected) {
			t.Errorf("trimNewline(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

// mockWriteCloser wraps a writer and adds Close functionality
type mockWriteCloser struct {
	*bytes.Buffer
	closed bool
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

// mockReadCloser wraps a reader and adds Close functionality
type mockReadCloser struct {
	io.Reader
	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestPipeTransport_Close(t *testing.T) {
	stdin := &mockWriteCloser{Buffer: &bytes.Buffer{}}
	stdout := &mockReadCloser{Reader: strings.NewReader("")}

	transport := NewPipeTransport(stdin, stdout)

	if err := transport.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !stdin.closed {
		t.Error("expected stdin to be closed")
	}
	if !stdout.closed {
		t.Error("expected stdout to be closed")
	}
}
