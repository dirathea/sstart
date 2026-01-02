package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Transport defines the interface for MCP message transport
type Transport interface {
	// ReadMessage reads the next JSON-RPC message from the transport
	ReadMessage() (*JSONRPCMessage, error)
	// WriteMessage writes a JSON-RPC message to the transport
	WriteMessage(msg *JSONRPCMessage) error
	// Close closes the transport
	Close() error
}

// StdioTransport implements the MCP stdio transport.
// Messages are newline-delimited JSON-RPC 2.0 messages.
type StdioTransport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
	closed  bool
	closeMu sync.RWMutex
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

// ReadMessage reads the next JSON-RPC message from stdin.
// Messages are expected to be newline-delimited JSON.
func (t *StdioTransport) ReadMessage() (*JSONRPCMessage, error) {
	t.closeMu.RLock()
	if t.closed {
		t.closeMu.RUnlock()
		return nil, fmt.Errorf("transport is closed")
	}
	t.closeMu.RUnlock()

	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// Remove trailing newline and any carriage return
	line = trimNewline(line)

	if len(line) == 0 {
		// Skip empty lines
		return t.ReadMessage()
	}

	var msg JSONRPCMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// WriteMessage writes a JSON-RPC message to stdout.
// The message is written as a single line of JSON followed by a newline.
func (t *StdioTransport) WriteMessage(msg *JSONRPCMessage) error {
	t.closeMu.RLock()
	if t.closed {
		t.closeMu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	t.closeMu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	// Write the JSON followed by a newline
	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if _, err := t.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// Close closes the transport
func (t *StdioTransport) Close() error {
	t.closeMu.Lock()
	defer t.closeMu.Unlock()
	t.closed = true
	return nil
}

// trimNewline removes trailing newline characters (LF and CRLF)
func trimNewline(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	if len(data) > 0 && data[len(data)-1] == '\r' {
		data = data[:len(data)-1]
	}
	return data
}

// PipeTransport is a bidirectional transport using io.Reader and io.Writer.
// This is useful for communicating with subprocess MCP servers via their stdin/stdout.
type PipeTransport struct {
	*StdioTransport
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// NewPipeTransport creates a new pipe transport for subprocess communication
func NewPipeTransport(stdin io.WriteCloser, stdout io.ReadCloser) *PipeTransport {
	return &PipeTransport{
		StdioTransport: NewStdioTransport(stdout, stdin),
		stdin:          stdin,
		stdout:         stdout,
	}
}

// Close closes both the stdin and stdout pipes
func (t *PipeTransport) Close() error {
	t.StdioTransport.Close()

	var errs []error
	if err := t.stdin.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close stdin: %w", err))
	}
	if err := t.stdout.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close stdout: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing pipe transport: %v", errs)
	}
	return nil
}
