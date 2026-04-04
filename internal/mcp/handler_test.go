package mcp

import (
	"encoding/json"
	"testing"
)

func newTestHandler() *Handler {
	return NewHandler("192.168.1.118", "Test Printer", nil)
}

func TestHandleInitialize(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if initResult.ProtocolVersion != ProtocolVersion {
		t.Errorf("expected protocol version %q, got %q", ProtocolVersion, initResult.ProtocolVersion)
	}
	if initResult.ServerInfo.Name != "go-mcp-brother-label-printer-direct" {
		t.Errorf("expected server name 'go-mcp-brother-label-printer-direct', got %q", initResult.ServerInfo.Name)
	}
	if initResult.Capabilities.Tools == nil {
		t.Error("expected tools capability")
	}
}

func TestHandleToolsList(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Tools) != 10 {
		t.Errorf("expected 10 tools, got %d", len(result.Tools))
	}

	// Verify expected tool names
	expectedTools := map[string]bool{
		"get_printer_info":   false,
		"get_supply_levels":  false,
		"print_label":        false,
		"print_label_image":  false,
		"print_table_label":  false,
		"print_big_label":    false,
		"get_print_queue":    false,
		"get_job_status":     false,
		"cancel_job":        false,
		"test_connectivity": false,
	}

	for _, tool := range result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestHandleResourcesList(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result struct {
		Resources []Resource `json:"resources"`
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Resources) != 3 {
		t.Errorf("expected 3 resources, got %d", len(result.Resources))
	}
}

func TestHandlePromptsList(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":4,"method":"prompts/list"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var result struct {
		Prompts []Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Prompts) != 3 {
		t.Errorf("expected 3 prompts, got %d", len(result.Prompts))
	}
}

func TestHandlePing(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":5,"method":"ping"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":6,"method":"unknown/method"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if rpcResp.Error.Code != MethodNotFound {
		t.Errorf("expected MethodNotFound code %d, got %d", MethodNotFound, rpcResp.Error.Code)
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	h := newTestHandler()

	resp, err := h.HandleRequest([]byte("not json"))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error == nil {
		t.Fatal("expected parse error")
	}
	if rpcResp.Error.Code != ParseError {
		t.Errorf("expected ParseError code %d, got %d", ParseError, rpcResp.Error.Code)
	}
}

func TestHandleNotification(t *testing.T) {
	h := newTestHandler()

	// Notification has no ID
	req := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	if resp != nil {
		t.Error("expected nil response for notification")
	}
}

func TestToolCallPrintLabelMissingArgs(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"print_label","arguments":{}}}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if rpcResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", rpcResp.Error)
	}

	// The tool result should be an error result
	resultBytes, _ := json.Marshal(rpcResp.Result)
	var toolResult ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}

	if !toolResult.IsError {
		t.Error("expected tool error for missing text argument")
	}
}

func TestToolCallUnknownTool(t *testing.T) {
	h := newTestHandler()

	req := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"nonexistent_tool","arguments":{}}}`
	resp, err := h.HandleRequest([]byte(req))
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(resp, &rpcResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	resultBytes, _ := json.Marshal(rpcResp.Result)
	var toolResult ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}

	if !toolResult.IsError {
		t.Error("expected tool error for unknown tool")
	}
}
