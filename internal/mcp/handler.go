package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"go-mcp-brother-label-printer-direct/internal/printer"
)

// Handler processes MCP JSON-RPC requests using direct printer communication.
type Handler struct {
	ippClient   *printer.IPPClient
	snmpClient  *printer.SNMPClient
	printerIP   string
	printerName string
	dialFunc    func(network, addr string) (net.Conn, error)
	tools       []Tool
	resources   []Resource
	prompts     []Prompt
}

// NewHandler creates a new MCP handler.
func NewHandler(printerIP, printerName string, dialFunc func(network, addr string) (net.Conn, error)) *Handler {
	h := &Handler{
		ippClient:   printer.NewIPPClient(printerIP, dialFunc),
		snmpClient:  printer.NewSNMPClient(printerIP, dialFunc),
		printerIP:   printerIP,
		printerName: printerName,
		dialFunc:    dialFunc,
	}

	h.registerTools()
	h.registerResources()
	h.registerPrompts()

	return h
}

// HandleRequest processes a raw HTTP request body as an MCP JSON-RPC request.
func (h *Handler) HandleRequest(body []byte) ([]byte, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return h.errorResponse(nil, ParseError, "Parse error")
	}

	if req.JSONRPC != "2.0" {
		return h.errorResponse(req.ID, InvalidRequest, "Invalid JSON-RPC version")
	}

	// Notifications (no ID) don't require a response
	if req.ID == nil || string(req.ID) == "null" {
		return nil, nil
	}

	switch req.Method {
	case "initialize":
		return h.handleInitialize(req.ID)
	case "tools/list":
		return h.handleToolsList(req.ID)
	case "tools/call":
		return h.handleToolsCall(req.ID, req.Params)
	case "resources/list":
		return h.handleResourcesList(req.ID)
	case "resources/read":
		return h.handleResourcesRead(req.ID, req.Params)
	case "prompts/list":
		return h.handlePromptsList(req.ID)
	case "prompts/get":
		return h.handlePromptsGet(req.ID, req.Params)
	case "ping":
		return h.successResponse(req.ID, map[string]string{})
	default:
		return h.errorResponse(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (h *Handler) handleInitialize(id json.RawMessage) ([]byte, error) {
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
			Prompts:   &PromptsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "go-mcp-brother-label-printer-direct",
			Version: "1.0.0",
		},
	}
	return h.successResponse(id, result)
}

func (h *Handler) handleToolsList(id json.RawMessage) ([]byte, error) {
	return h.successResponse(id, map[string]interface{}{
		"tools": h.tools,
	})
}

func (h *Handler) handleToolsCall(id json.RawMessage, params json.RawMessage) ([]byte, error) {
	var call ToolCallParams
	if err := json.Unmarshal(params, &call); err != nil {
		return h.errorResponse(id, InvalidParams, "Invalid tool call parameters")
	}

	slog.Info("tool call", "name", call.Name, "arguments", call.Arguments)

	result := h.dispatchTool(call.Name, call.Arguments)
	return h.successResponse(id, result)
}

func (h *Handler) handleResourcesList(id json.RawMessage) ([]byte, error) {
	return h.successResponse(id, map[string]interface{}{
		"resources": h.resources,
	})
}

func (h *Handler) handleResourcesRead(id json.RawMessage, params json.RawMessage) ([]byte, error) {
	var readParams ResourceReadParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return h.errorResponse(id, InvalidParams, "Invalid resource read parameters")
	}

	result := h.readResource(readParams.URI)
	return h.successResponse(id, result)
}

func (h *Handler) handlePromptsList(id json.RawMessage) ([]byte, error) {
	return h.successResponse(id, map[string]interface{}{
		"prompts": h.prompts,
	})
}

func (h *Handler) handlePromptsGet(id json.RawMessage, params json.RawMessage) ([]byte, error) {
	var getParams PromptGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return h.errorResponse(id, InvalidParams, "Invalid prompt get parameters")
	}

	result := h.getPrompt(getParams.Name, getParams.Arguments)
	return h.successResponse(id, result)
}

// --- Tool dispatch ---

func (h *Handler) dispatchTool(name string, args map[string]interface{}) *ToolResult {
	switch name {
	case "get_printer_info":
		return h.toolGetPrinterInfo()
	case "get_supply_levels":
		return h.toolGetSupplyLevels()
	case "print_label":
		return h.toolPrintLabel(args)
	case "print_label_image":
		return h.toolPrintLabelImage(args)
	case "get_print_queue":
		return h.toolGetPrintQueue()
	case "get_job_status":
		return h.toolGetJobStatus(args)
	case "cancel_job":
		return h.toolCancelJob(args)
	case "test_connectivity":
		return h.toolTestConnectivity()
	default:
		return ErrorResult(fmt.Sprintf("Unknown tool: %s", name))
	}
}

func (h *Handler) toolGetPrinterInfo() *ToolResult {
	info, err := h.ippClient.GetPrinterInfo()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get printer info: %v", err))
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	return TextResult(string(data))
}

func (h *Handler) toolGetSupplyLevels() *ToolResult {
	status, err := h.snmpClient.GetSupplyLevels()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get supply levels: %v", err))
	}
	data, _ := json.MarshalIndent(status, "", "  ")
	return TextResult(string(data))
}

func (h *Handler) toolPrintLabel(args map[string]interface{}) *ToolResult {
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return ErrorResult("'text' argument is required")
	}

	jobName, _ := args["job_name"].(string)
	copies := 1
	if c, ok := args["copies"].(float64); ok {
		copies = int(c)
	}

	result, err := h.ippClient.PrintText(text, jobName, copies)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Print failed: %v", err))
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return TextResult(string(data))
}

func (h *Handler) toolPrintLabelImage(args map[string]interface{}) *ToolResult {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return ErrorResult("'url' argument is required")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return ErrorResult("URL must start with http:// or https://")
	}

	transport := &http.Transport{}
	if h.dialFunc != nil {
		transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return h.dialFunc(network, addr)
		}
	}
	dlClient := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	resp, err := dlClient.Get(url)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to download image: %v", err))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to read image content: %v", err))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if idx := strings.Index(contentType, ";"); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	jobName, _ := args["job_name"].(string)
	if jobName == "" {
		jobName = "Label Image"
	}
	copies := 1
	if c, ok := args["copies"].(float64); ok {
		copies = int(c)
	}

	result, err := h.ippClient.PrintDocument(data, contentType, jobName, copies)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Print failed: %v", err))
	}
	resultData, _ := json.MarshalIndent(result, "", "  ")
	return TextResult(string(resultData))
}

func (h *Handler) toolGetPrintQueue() *ToolResult {
	jobs, err := h.ippClient.GetJobs()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get print queue: %v", err))
	}
	if len(jobs) == 0 {
		return TextResult("Print queue is empty")
	}
	data, _ := json.MarshalIndent(jobs, "", "  ")
	return TextResult(string(data))
}

func (h *Handler) toolGetJobStatus(args map[string]interface{}) *ToolResult {
	jobIDFloat, ok := args["job_id"].(float64)
	if !ok {
		return ErrorResult("'job_id' argument is required (integer)")
	}
	jobID := int(jobIDFloat)

	job, err := h.ippClient.GetJobStatus(jobID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get job status: %v", err))
	}
	data, _ := json.MarshalIndent(job, "", "  ")
	return TextResult(string(data))
}

func (h *Handler) toolCancelJob(args map[string]interface{}) *ToolResult {
	jobIDFloat, ok := args["job_id"].(float64)
	if !ok {
		return ErrorResult("'job_id' argument is required (integer)")
	}
	jobID := int(jobIDFloat)

	if err := h.ippClient.CancelJob(jobID); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to cancel job: %v", err))
	}
	return TextResult(fmt.Sprintf("Job %d cancelled successfully", jobID))
}

func (h *Handler) toolTestConnectivity() *ToolResult {
	result := printer.TestConnectivity(h.printerIP, h.dialFunc)
	data, _ := json.MarshalIndent(result, "", "  ")
	return TextResult(string(data))
}

// --- Resource dispatch ---

func (h *Handler) readResource(uri string) *ResourceReadResult {
	switch uri {
	case "printer://info":
		info, err := h.ippClient.GetPrinterInfo()
		if err != nil {
			return &ResourceReadResult{
				Contents: []ResourceContent{{URI: uri, MimeType: "text/plain", Text: fmt.Sprintf("Error: %v", err)}},
			}
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return &ResourceReadResult{
			Contents: []ResourceContent{{URI: uri, MimeType: "application/json", Text: string(data)}},
		}

	case "printer://supplies":
		status, err := h.snmpClient.GetSupplyLevels()
		if err != nil {
			return &ResourceReadResult{
				Contents: []ResourceContent{{URI: uri, MimeType: "text/plain", Text: fmt.Sprintf("Error: %v", err)}},
			}
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return &ResourceReadResult{
			Contents: []ResourceContent{{URI: uri, MimeType: "application/json", Text: string(data)}},
		}

	case "printer://help":
		return &ResourceReadResult{
			Contents: []ResourceContent{{URI: uri, MimeType: "text/markdown", Text: h.helpText()}},
		}

	default:
		return &ResourceReadResult{
			Contents: []ResourceContent{{URI: uri, MimeType: "text/plain", Text: "Resource not found: " + uri}},
		}
	}
}

func (h *Handler) helpText() string {
	return fmt.Sprintf(`# Brother Label Printer MCP Server - Help

## Printer
- **Model:** %s
- **IP:** %s
- **Type:** P-touch label maker with TZe tape cartridges

## Available Tools

### get_printer_info
Get label printer model, status, capabilities, and supported media.

### get_supply_levels
Check tape cartridge levels and status via SNMP.

### print_label
Print a text label. The PT-P750W will auto-size the label to fit the text content. Arguments:
- **text** (required): The text to print on the label
- **job_name** (optional): Name for the print job
- **copies** (optional): Number of copies (default: 1)

### print_label_image
Download an image from a URL and print it as a label. Arguments:
- **url** (required): URL to download the image (http:// or https://)
- **job_name** (optional): Name for the print job
- **copies** (optional): Number of copies (default: 1)

### get_print_queue
View current print jobs in the queue.

### get_job_status
Check the status of a specific print job. Arguments:
- **job_id** (required): The job ID to check

### cancel_job
Cancel a print job. Arguments:
- **job_id** (required): The job ID to cancel

### test_connectivity
Test network connectivity to the label printer (IPP, HTTP, SNMP ports).

## Supported Protocols
- **IPP** (port 631): Print jobs, queue management, printer status
- **SNMP** (port 161): Tape cartridge supply levels
- **HTTP** (port 80): Printer web interface

## Tape Information
The PT-P750W uses TZe laminated tape cartridges in widths: 3.5mm, 6mm, 9mm, 12mm, 18mm, 24mm.
The printer auto-cuts labels after printing.
`, h.printerName, h.printerIP)
}

// --- Prompt dispatch ---

func (h *Handler) getPrompt(name string, args map[string]string) *PromptGetResult {
	switch name {
	case "diagnose-printer":
		return &PromptGetResult{
			Description: "Diagnose label printer issues",
			Messages: []PromptMessage{
				{Role: "user", Content: ContentBlock{Type: "text", Text: fmt.Sprintf(
					`Please diagnose the Brother label printer at %s (%s). Follow these steps:
1. Run test_connectivity to check all ports
2. Run get_printer_info to check printer state and any error reasons
3. Run get_supply_levels to check tape cartridge status
4. Run get_print_queue to check for stuck jobs
5. Summarize findings and suggest fixes for any issues found`, h.printerIP, h.printerName)}},
			},
		}

	case "supply-check":
		return &PromptGetResult{
			Description: "Check tape cartridge supply levels",
			Messages: []PromptMessage{
				{Role: "user", Content: ContentBlock{Type: "text", Text: fmt.Sprintf(
					`Check the supply levels for the Brother label printer at %s (%s):
1. Run get_supply_levels to get current tape cartridge status
2. Report the installed tape type, width, and remaining level
3. Flag any supplies below 20%% as low
4. Flag any supplies below 10%% as critical`, h.printerIP, h.printerName)}},
			},
		}

	case "print-label":
		text := args["text"]
		if text == "" {
			text = "[specify label text]"
		}
		return &PromptGetResult{
			Description: "Print a label with smart defaults",
			Messages: []PromptMessage{
				{Role: "user", Content: ContentBlock{Type: "text", Text: fmt.Sprintf(
					`Print a label with text: %s
1. First check printer status with get_printer_info
2. Use print_label to send the text
3. Verify the job was accepted by checking get_print_queue
4. Report the job ID and status`, text)}},
			},
		}

	default:
		return &PromptGetResult{
			Messages: []PromptMessage{
				{Role: "user", Content: ContentBlock{Type: "text", Text: "Unknown prompt: " + name}},
			},
		}
	}
}

// --- Tool/resource/prompt registration ---

func (h *Handler) registerTools() {
	h.tools = []Tool{
		{
			Name:        "get_printer_info",
			Description: fmt.Sprintf("Get detailed information about the Brother label printer at %s including model, status, capabilities, and supported tape media", h.printerIP),
			InputSchema: InputSchema{Type: "object"},
			Annotations: &ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
		},
		{
			Name:        "get_supply_levels",
			Description: "Get tape cartridge supply levels for the Brother label printer via SNMP",
			InputSchema: InputSchema{Type: "object"},
			Annotations: &ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
		},
		{
			Name:        "print_label",
			Description: "Print a text label on the Brother PT-P750W. The printer auto-sizes and auto-cuts the label.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"text":     {Type: "string", Description: "The text content to print on the label"},
					"job_name": {Type: "string", Description: "Name for the print job (optional)"},
					"copies":   {Type: "integer", Description: "Number of label copies to print", Minimum: intPtr(1), Maximum: intPtr(99), Default: 1},
				},
				Required: []string{"text"},
			},
			Annotations: &ToolAnnotations{DestructiveHint: BoolPtr(true)},
		},
		{
			Name:        "print_label_image",
			Description: "Download an image from a URL and print it as a label. Supports PNG and JPEG images.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url":      {Type: "string", Description: "URL of the image to download and print as a label (http:// or https://)"},
					"job_name": {Type: "string", Description: "Name for the print job (optional)"},
					"copies":   {Type: "integer", Description: "Number of label copies to print", Minimum: intPtr(1), Maximum: intPtr(99), Default: 1},
				},
				Required: []string{"url"},
			},
			Annotations: &ToolAnnotations{DestructiveHint: BoolPtr(true)},
		},
		{
			Name:        "get_print_queue",
			Description: "Get the current print queue showing all active and pending label print jobs",
			InputSchema: InputSchema{Type: "object"},
			Annotations: &ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
		},
		{
			Name:        "get_job_status",
			Description: "Get the status of a specific print job by ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"job_id": {Type: "integer", Description: "The print job ID to check"},
				},
				Required: []string{"job_id"},
			},
			Annotations: &ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
		},
		{
			Name:        "cancel_job",
			Description: "Cancel a specific print job by ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"job_id": {Type: "integer", Description: "The print job ID to cancel"},
				},
				Required: []string{"job_id"},
			},
			Annotations: &ToolAnnotations{DestructiveHint: BoolPtr(true)},
		},
		{
			Name:        "test_connectivity",
			Description: fmt.Sprintf("Test network connectivity to the Brother label printer at %s on IPP (631), HTTP (80), and SNMP (161) ports", h.printerIP),
			InputSchema: InputSchema{Type: "object"},
			Annotations: &ToolAnnotations{ReadOnlyHint: BoolPtr(true)},
		},
	}
}

func (h *Handler) registerResources() {
	h.resources = []Resource{
		{
			URI:         "printer://info",
			Name:        "Label Printer Information",
			Description: "Current Brother label printer status, model, and capabilities",
			MimeType:    "application/json",
		},
		{
			URI:         "printer://supplies",
			Name:        "Tape Cartridge Levels",
			Description: "Current tape cartridge supply levels",
			MimeType:    "application/json",
		},
		{
			URI:         "printer://help",
			Name:        "Label Printing Guide",
			Description: "Help guide for using the Brother label printer MCP tools",
			MimeType:    "text/markdown",
		},
	}
}

func (h *Handler) registerPrompts() {
	h.prompts = []Prompt{
		{
			Name:        "diagnose-printer",
			Description: "Run a full diagnostic on the Brother label printer: connectivity, status, tape levels, and queue",
		},
		{
			Name:        "supply-check",
			Description: "Check tape cartridge supply levels and flag low or critical supplies",
		},
		{
			Name:        "print-label",
			Description: "Print a text label with smart defaults",
			Arguments: []PromptArgument{
				{Name: "text", Description: "Text to print on the label", Required: true},
			},
		},
	}
}

// --- Response helpers ---

func (h *Handler) successResponse(id json.RawMessage, result interface{}) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

func (h *Handler) errorResponse(id json.RawMessage, code int, message string) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	return json.Marshal(resp)
}

func intPtr(i int) *int {
	return &i
}
