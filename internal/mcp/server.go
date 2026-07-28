package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/version"
)

// protocolVersion is the MCP revision this server implements. When a client
// asks for a different revision we still answer with ours; the client decides
// whether it can proceed.
const protocolVersion = "2025-06-18"

const serverName = "aiah"

// JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// maxRequestBytes bounds a single request line. Requests carry paths and short
// option lists, never payloads, so anything larger is a malformed or hostile
// stream rather than legitimate traffic.
const maxRequestBytes = 1 << 20

// userHomeDir is indirected so tests can exercise the "no home given" path.
var userHomeDir = os.UserHomeDir

// Options configures a server run. In and Out carry the newline-delimited
// JSON-RPC stream; ErrOut receives diagnostics only, because anything written
// to Out that is not a JSON-RPC message corrupts the transport.
type Options struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run serves MCP over the given streams until In is exhausted.
//
// Transport is newline-delimited JSON-RPC 2.0, the MCP stdio framing. A request
// without an id is a notification and gets no reply, per JSON-RPC.
func Run(options Options) error {
	if options.In == nil || options.Out == nil {
		return errors.New("mcp: in and out are required")
	}
	errOut := options.ErrOut
	if errOut == nil {
		errOut = io.Discard
	}

	reader := bufio.NewReaderSize(options.In, 64*1024)
	registry := toolsByName()

	for {
		line, err := readLine(reader)
		if errors.Is(err, io.EOF) {
			if strings.TrimSpace(string(line)) == "" {
				return nil
			}
		} else if err != nil {
			if errors.Is(err, errRequestTooLarge) {
				// The stream position is no longer trustworthy after an
				// oversized line, so report and stop rather than resync onto
				// what may be the middle of a message.
				_, _ = fmt.Fprintln(errOut, "aiah mcp: request exceeds size limit")
				return err
			}
			return err
		}

		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			if writeErr := handleLine(trimmed, registry, options.Out); writeErr != nil {
				return writeErr
			}
		}

		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

var errRequestTooLarge = errors.New("mcp: request too large")

func readLine(reader *bufio.Reader) ([]byte, error) {
	line := make([]byte, 0, 512)
	for {
		chunk, err := reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxRequestBytes {
			return nil, errRequestTooLarge
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return line, err
	}
}

func handleLine(line string, registry map[string]Tool, out io.Writer) error {
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return writeResponse(out, response{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &responseError{Code: codeParseError, Message: "parse error"},
		})
	}

	// No id means notification: do the work if we know it, answer nothing.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil
	}

	if req.JSONRPC != "2.0" {
		return writeResponse(out, response{
			JSONRPC: "2.0", ID: req.ID,
			Error: &responseError{Code: codeInvalidRequest, Message: "jsonrpc must be \"2.0\""},
		})
	}

	result, rpcErr := dispatch(req, registry)
	if rpcErr != nil {
		return writeResponse(out, response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr})
	}
	return writeResponse(out, response{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func dispatch(req request, registry map[string]Tool) (any, *responseError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{
				"name":    serverName,
				"version": version.Version,
			},
			// Stated in-band as well as in the docs: a client that surfaces
			// instructions to a model should say what this server cannot do.
			"instructions": "Read-only access to aiah. This server can inspect assets and " +
				"deployments but cannot apply packages, roll back, or write any file.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDescriptors()}, nil
	case "tools/call":
		return callTool(req.Params, registry)
	default:
		return nil, &responseError{Code: codeMethodNotFound, Message: "unknown method: " + req.Method}
	}
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func callTool(rawParams json.RawMessage, registry map[string]Tool) (any, *responseError) {
	var params callParams
	if len(rawParams) > 0 {
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &responseError{Code: codeInvalidParams, Message: "invalid params"}
		}
	}
	if params.Name == "" {
		return nil, &responseError{Code: codeInvalidParams, Message: "tool name is required"}
	}
	tool, ok := registry[params.Name]
	if !ok {
		// Unknown tool is a tool-level failure, not a protocol failure: the
		// model should see it and pick a different tool.
		return toolFailure("unknown tool: " + params.Name), nil
	}

	report, err := tool.Handler(params.Arguments)
	if err != nil {
		return toolFailure(err.Error()), nil
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return toolFailure("cannot encode report"), nil
	}
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": string(encoded)}},
	}, nil
}

func toolFailure(message string) map[string]any {
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": message}},
		"isError": true,
	}
}

func toolDescriptors() []any {
	tools := Tools()
	out := make([]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}
	return out
}

func toolsByName() map[string]Tool {
	tools := Tools()
	registry := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		registry[tool.Name] = tool
	}
	return registry
}

func writeResponse(out io.Writer, value response) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = out.Write(encoded)
	return err
}

func resolveHome(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: cannot determine home directory", errInvalidArguments)
	}
	return home, nil
}
