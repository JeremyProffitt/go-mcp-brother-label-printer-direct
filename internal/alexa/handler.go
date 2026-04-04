package alexa

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"go-mcp-brother-label-printer-direct/internal/printer"
	"go-mcp-brother-label-printer-direct/internal/token"
)

// Handler processes Alexa Skills Kit requests using direct printer communication.
type Handler struct {
	ippClient   *printer.IPPClient
	snmpClient  *printer.SNMPClient
	printerIP   string
	printerName string
	skillID     string
	keyPair     *token.KeyPair
	dialFunc    func(network, addr string) (net.Conn, error)
}

// NewHandler creates a new Alexa handler.
func NewHandler(printerIP, printerName, skillID string, keyPair *token.KeyPair, dialFunc func(network, addr string) (net.Conn, error)) *Handler {
	return &Handler{
		ippClient:   printer.NewIPPClient(printerIP, dialFunc),
		snmpClient:  printer.NewSNMPClient(printerIP, dialFunc),
		printerIP:   printerIP,
		printerName: printerName,
		skillID:     skillID,
		keyPair:     keyPair,
		dialFunc:    dialFunc,
	}
}

// HandleRequest processes a raw Alexa Skills Kit request and returns a response.
func (h *Handler) HandleRequest(body []byte) ([]byte, error) {
	var req AlexaRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse alexa request: %w", err)
	}

	// Validate skill ID
	if h.skillID != "" && req.Session != nil {
		if req.Session.Application.ApplicationID != h.skillID {
			slog.Warn("alexa request rejected: skill ID mismatch",
				"expected", h.skillID,
				"got", req.Session.Application.ApplicationID,
			)
			return nil, fmt.Errorf("invalid application ID")
		}
	}

	// Validate access token (account linking)
	if req.Session != nil && req.Session.User.AccessToken != "" {
		_, err := token.ValidateAccessToken(h.keyPair, req.Session.User.AccessToken)
		if err != nil {
			slog.Warn("alexa access token invalid", "error", err)
			return json.Marshal(linkAccountResponse())
		}
	} else if req.Request != nil && req.Request.Type == "IntentRequest" {
		// Require account linking for intent requests
		return json.Marshal(linkAccountResponse())
	}

	if req.Request == nil {
		return json.Marshal(plainTextResponse("Something went wrong.", true))
	}

	slog.Info("alexa request", "type", req.Request.Type,
		"intent", intentName(req.Request),
		"session_new", req.Session != nil && req.Session.New,
	)

	var resp *AlexaResponse
	switch req.Request.Type {
	case "LaunchRequest":
		resp = h.handleLaunch()
	case "IntentRequest":
		resp = h.handleIntent(&req)
	case "SessionEndedRequest":
		resp = plainTextResponse("", true)
	default:
		resp = plainTextResponse("I'm not sure how to handle that.", true)
	}

	return json.Marshal(resp)
}

func (h *Handler) handleLaunch() *AlexaResponse {
	speech := fmt.Sprintf(
		"Welcome to My Printer. Your %s is connected. "+
			"You can ask me to check printer status, check ink levels, "+
			"print text, check the print queue, or test connectivity. "+
			"What would you like to do?",
		h.printerName,
	)
	return &AlexaResponse{
		Version: "1.0",
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: speech,
			},
			Card: &Card{
				Type:    "Simple",
				Title:   "My Printer",
				Content: "Connected to " + h.printerName,
			},
			Reprompt: &Reprompt{
				OutputSpeech: OutputSpeech{
					Type: "PlainText",
					Text: "What would you like to do? You can say check ink levels, or printer status.",
				},
			},
			ShouldEndSession: boolPtr(false),
		},
	}
}

func (h *Handler) handleIntent(req *AlexaRequest) *AlexaResponse {
	if req.Request.Intent == nil {
		return plainTextResponse("I didn't understand that request.", true)
	}

	name := req.Request.Intent.Name

	// Check if this is a confirmation response for a pending action
	if name == "AMAZON.YesIntent" {
		return h.handleConfirmYes(req)
	}
	if name == "AMAZON.NoIntent" {
		return h.handleConfirmNo()
	}

	switch name {
	case "GetPrinterStatusIntent":
		return h.handleGetPrinterStatus()
	case "GetInkLevelsIntent":
		return h.handleGetInkLevels()
	case "PrintTextIntent":
		return h.handlePrintText(req)
	case "GetPrintQueueIntent":
		return h.handleGetPrintQueue()
	case "GetJobStatusIntent":
		return h.handleGetJobStatus(req)
	case "CancelJobIntent":
		return h.handleCancelJob(req)
	case "TestConnectivityIntent":
		return h.handleTestConnectivity()
	case "AMAZON.HelpIntent":
		return h.handleHelp()
	case "AMAZON.StopIntent", "AMAZON.CancelIntent":
		return plainTextResponse("Goodbye!", true)
	case "AMAZON.FallbackIntent":
		return plainTextResponse(
			"I'm not sure how to help with that. You can ask me to check printer status, check ink levels, or print text.",
			false,
		)
	default:
		return plainTextResponse("I don't know how to do that yet.", true)
	}
}

func (h *Handler) handleHelp() *AlexaResponse {
	speech := "Here's what I can do. " +
		"Say check printer status to see if the printer is online. " +
		"Say check ink levels to see toner supply levels. " +
		"Say print followed by your text to print something. " +
		"Say check the print queue to see pending jobs. " +
		"Say check job followed by a number to get a job's status. " +
		"Say cancel job followed by a number to cancel a print job. " +
		"Say test connectivity to check all printer ports. " +
		"What would you like to do?"
	return &AlexaResponse{
		Version: "1.0",
		Response: ResponseBody{
			OutputSpeech: &OutputSpeech{
				Type: "PlainText",
				Text: speech,
			},
			Reprompt: &Reprompt{
				OutputSpeech: OutputSpeech{
					Type: "PlainText",
					Text: "What would you like to do?",
				},
			},
			ShouldEndSession: boolPtr(false),
		},
	}
}

func intentName(r *Request) string {
	if r == nil || r.Intent == nil {
		return ""
	}
	return r.Intent.Name
}
