// Package mcp implements the Model Context Protocol (MCP) proxy functionality.
// It provides types and utilities for JSON-RPC 2.0 communication and MCP message handling.
package mcp

import (
	"encoding/json"
	"fmt"
)

const (
	// JSONRPCVersion is the JSON-RPC protocol version
	JSONRPCVersion = "2.0"

	// MCPProtocolVersion is the MCP protocol version supported by this implementation
	MCPProtocolVersion = "2024-11-05"
)

// JSON-RPC 2.0 Error Codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// RequestID represents a JSON-RPC request ID which can be string, number, or null
type RequestID struct {
	value interface{}
}

// NewRequestID creates a new RequestID from a value
func NewRequestID(v interface{}) RequestID {
	return RequestID{value: v}
}

// Value returns the underlying value of the RequestID
func (id RequestID) Value() interface{} {
	return id.value
}

// IsNull returns true if the RequestID is null/nil
func (id RequestID) IsNull() bool {
	return id.value == nil
}

// MarshalJSON implements json.Marshaler
func (id RequestID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.value)
}

// UnmarshalJSON implements json.Unmarshaler
func (id *RequestID) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &id.value)
}

// JSONRPCMessage represents a generic JSON-RPC 2.0 message (request, response, or notification)
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// IsRequest returns true if this is a JSON-RPC request (has method and id)
func (m *JSONRPCMessage) IsRequest() bool {
	return m.Method != "" && m.ID != nil && !m.ID.IsNull()
}

// IsNotification returns true if this is a JSON-RPC notification (has method, no id)
func (m *JSONRPCMessage) IsNotification() bool {
	return m.Method != "" && (m.ID == nil || m.ID.IsNull())
}

// IsResponse returns true if this is a JSON-RPC response (has result or error)
func (m *JSONRPCMessage) IsResponse() bool {
	return m.Result != nil || m.Error != nil
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface
func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewJSONRPCRequest creates a new JSON-RPC request message
func NewJSONRPCRequest(id interface{}, method string, params interface{}) (*JSONRPCMessage, error) {
	var paramsRaw json.RawMessage
	if params != nil {
		var err error
		paramsRaw, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	reqID := NewRequestID(id)
	return &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      &reqID,
		Method:  method,
		Params:  paramsRaw,
	}, nil
}

// NewJSONRPCNotification creates a new JSON-RPC notification message (no id)
func NewJSONRPCNotification(method string, params interface{}) (*JSONRPCMessage, error) {
	var paramsRaw json.RawMessage
	if params != nil {
		var err error
		paramsRaw, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	return &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  paramsRaw,
	}, nil
}

// NewJSONRPCResponse creates a new JSON-RPC response message
func NewJSONRPCResponse(id interface{}, result interface{}) (*JSONRPCMessage, error) {
	var resultRaw json.RawMessage
	if result != nil {
		var err error
		resultRaw, err = json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
	}

	reqID := NewRequestID(id)
	return &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      &reqID,
		Result:  resultRaw,
	}, nil
}

// NewJSONRPCErrorResponse creates a new JSON-RPC error response message
func NewJSONRPCErrorResponse(id interface{}, code int, message string, data interface{}) (*JSONRPCMessage, error) {
	var dataRaw json.RawMessage
	if data != nil {
		var err error
		dataRaw, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal error data: %w", err)
		}
	}

	reqID := NewRequestID(id)
	return &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      &reqID,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    dataRaw,
		},
	}, nil
}

// MCP Message Types

// Info represents client or server information
type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities represents client or server capabilities
type Capabilities struct {
	// Client capabilities
	Roots       *RootsCapability       `json:"roots,omitempty"`
	Sampling    *SamplingCapability    `json:"sampling,omitempty"`
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`

	// Server capabilities
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// RootsCapability represents the roots capability
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents the sampling capability
type SamplingCapability struct{}

// ElicitationCapability represents the elicitation capability
type ElicitationCapability struct{}

// ToolsCapability represents the tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents the resources capability
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability represents the prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability represents the logging capability
type LoggingCapability struct{}

// InitializeParams represents the parameters for the initialize request
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      Info         `json:"clientInfo"`
}

// InitializeResult represents the result of the initialize request
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      Info         `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult represents the result of tools/list
type ToolsListResult struct {
	Tools      []Tool  `json:"tools"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ToolCallParams represents the parameters for tools/call
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult represents the result of tools/call
type ToolCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content in tool results
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // Base64 encoded for binary
	// For embedded resources
	Resource *ResourceContent `json:"resource,omitempty"`
}

// ResourceContent represents embedded resource content
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // Base64 encoded
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult represents the result of resources/list
type ResourcesListResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor *string    `json:"nextCursor,omitempty"`
}

// ResourcesReadParams represents the parameters for resources/read
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult represents the result of resources/read
type ResourcesReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceTemplate represents an MCP resource template
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceTemplatesListResult represents the result of resources/templates/list
type ResourceTemplatesListResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	NextCursor        *string            `json:"nextCursor,omitempty"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListResult represents the result of prompts/list
type PromptsListResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor *string  `json:"nextCursor,omitempty"`
}

// PromptsGetParams represents the parameters for prompts/get
type PromptsGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptMessage represents a message in a prompt
type PromptMessage struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// PromptsGetResult represents the result of prompts/get
type PromptsGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PaginatedRequest represents a request with pagination
type PaginatedRequest struct {
	Cursor *string `json:"cursor,omitempty"`
}

// MCP Method names
const (
	MethodInitialize             = "initialize"
	MethodInitialized            = "notifications/initialized"
	MethodToolsList              = "tools/list"
	MethodToolsCall              = "tools/call"
	MethodResourcesList          = "resources/list"
	MethodResourcesRead          = "resources/read"
	MethodResourcesTemplatesList = "resources/templates/list"
	MethodPromptsList            = "prompts/list"
	MethodPromptsGet             = "prompts/get"
	MethodPing                   = "ping"
	MethodCancelled              = "notifications/cancelled"
	MethodProgress               = "notifications/progress"
)
