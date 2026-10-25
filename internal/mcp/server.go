package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// Server implements an MCP stdio JSON-RPC 2.0 server.
type Server struct {
	handler         *ToolHandler
	reader          io.Reader
	writer          io.Writer
	mu              sync.Mutex
	connectedClient *types.ConnectedClient
}

// NewServer creates a new MCP server.
func NewServer(handler *ToolHandler, reader io.Reader, writer io.Writer) *Server {
	return &Server{
		handler: handler,
		reader:  reader,
		writer:  writer,
	}
}

// Serve reads JSON-RPC requests from the input stream and writes responses to the output stream.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.reader)
	// Allow larger buffer for code diffs and payloads
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(ctx, &req)
	}

	return scanner.Err()
}

func (s *Server) handleRequest(ctx context.Context, req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		var initParams struct {
			ClientInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		_ = json.Unmarshal(req.Params, &initParams)

		clientName := initParams.ClientInfo.Name
		if clientName == "" {
			clientName = "unknown-agent"
		}

		s.mu.Lock()
		s.connectedClient = &types.ConnectedClient{
			ID:          fmt.Sprintf("client-%d", time.Now().UnixNano()),
			Name:        clientName,
			Version:     initParams.ClientInfo.Version,
			ConnectedAt: time.Now(),
			LastSeen:    time.Now(),
		}
		s.mu.Unlock()

		result := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "sandforge",
				"version": "1.0.0",
			},
		}
		s.sendResult(req.ID, result)

	case "notifications/initialized":
		// Acknowledgement notification, no response required

	case "tools/list":
		tools := GetAvailableTools()
		s.sendResult(req.ID, map[string]interface{}{
			"tools": tools,
		})

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "Invalid params")
			return
		}

		s.mu.Lock()
		if s.connectedClient != nil {
			s.connectedClient.LastSeen = time.Now()
			s.connectedClient.CallsCount++
		}
		s.mu.Unlock()

		result, err := s.handler.ExecuteTool(ctx, params.Name, params.Arguments)
		if err != nil {
			s.sendError(req.ID, -32603, fmt.Sprintf("Internal error: %v", err))
			return
		}

		s.sendResult(req.ID, result)

	case "ping":
		s.sendResult(req.ID, map[string]interface{}{})

	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// Client returns information about the connected client.
func (s *Server) Client() *types.ConnectedClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connectedClient
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	if id == nil {
		return
	}
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.writeMessage(resp)
}

func (s *Server) sendError(id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	s.writeMessage(resp)
}

func (s *Server) writeMessage(msg interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, _ = s.writer.Write(append(data, '\n'))
}
