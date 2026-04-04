package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	fiberadapter "github.com/awslabs/aws-lambda-go-api-proxy/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"go-mcp-brother-label-printer-direct/internal/alexa"
	appconfig "go-mcp-brother-label-printer-direct/internal/config"
	"go-mcp-brother-label-printer-direct/internal/mcp"
	"go-mcp-brother-label-printer-direct/internal/middleware"
	"go-mcp-brother-label-printer-direct/internal/oauth"
	"go-mcp-brother-label-printer-direct/internal/printer"
	"go-mcp-brother-label-printer-direct/internal/store"
	"go-mcp-brother-label-printer-direct/internal/telemetry"
	"go-mcp-brother-label-printer-direct/internal/token"
	"go-mcp-brother-label-printer-direct/internal/vpn"
)

var fiberLambda *fiberadapter.FiberLambda
var otelShutdown func(context.Context) error
var printerIPP *printer.IPPClient
var printerSNMP *printer.SNMPClient
var alexaHandler *alexa.Handler

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx := context.Background()

	cfg, err := appconfig.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("failed to load AWS config", "error", err)
		os.Exit(1)
	}

	smClient := secretsmanager.NewFromConfig(awsCfg)
	ddbClient := dynamodb.NewFromConfig(awsCfg)

	otelShutdown, err = telemetry.Init(ctx, cfg.OTelServiceName, cfg.OTelEndpoint)
	if err != nil {
		slog.Warn("OTel init failed, continuing without telemetry", "error", err)
	}

	keyPair, err := token.LoadKeyPair(ctx, smClient, cfg.JWTSigningKeyARN)
	if err != nil {
		slog.Error("failed to load JWT signing keys", "error", err)
		os.Exit(1)
	}

	// Initialize WireGuard tunnel (optional)
	var tunnel *vpn.Tunnel
	var dialFunc func(network, addr string) (net.Conn, error)

	wgCfg, err := vpn.LoadConfigFromEnv()
	if err != nil {
		slog.Warn("WireGuard VPN disabled", "reason", err)
	} else {
		tunnel, err = vpn.StartTunnel(wgCfg)
		if err != nil {
			slog.Error("failed to start WireGuard tunnel", "error", err)
		} else {
			slog.Info("WireGuard tunnel started successfully")
			dialFunc = tunnel.DialContext
		}
	}

	// Initialize DynamoDB store
	oauthStore := store.NewDynamoDBStore(ddbClient, cfg.DynamoDBTable)

	// Create OAuth handler
	oauthHandler := &oauth.Handler{
		Config:  cfg,
		Store:   oauthStore,
		KeyPair: keyPair,
	}

	// Create MCP handler with direct printer access
	mcpHandler := mcp.NewHandler(cfg.PrinterIP, cfg.PrinterName, dialFunc)

	// Printer clients for scheduled keepalive
	printerIPP = printer.NewIPPClient(cfg.PrinterIP, dialFunc)
	printerSNMP = printer.NewSNMPClient(cfg.PrinterIP, dialFunc)

	// Create Alexa handler (optional)
	if cfg.AlexaSkillID != "" {
		alexaHandler = alexa.NewHandler(cfg.PrinterIP, cfg.PrinterName, cfg.AlexaSkillID, keyPair, dialFunc)
		slog.Info("Alexa skill handler initialized", "skill_id", cfg.AlexaSkillID)
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:           29 * time.Second,
		WriteTimeout:          29 * time.Second,
		AppName:               "mcp-brother-label-printer-direct",
		DisableStartupMessage: true,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.OTelTracing())

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":     "ok",
			"vpn":        tunnel != nil,
			"printer_ip": cfg.PrinterIP,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
	})

	// OAuth routes
	oauthHandler.RegisterRoutes(app)

	// MCP endpoint (authenticated)
	app.Post("/mcp", oauth.BearerAuthMiddleware(keyPair), func(c *fiber.Ctx) error {
		body := c.Body()

		resp, err := mcpHandler.HandleRequest(body)
		if err != nil {
			slog.Error("MCP handler error", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		if resp == nil {
			return c.SendStatus(fiber.StatusNoContent)
		}

		c.Set("Content-Type", "application/json")
		return c.Send(resp)
	})

	// Direct MCP endpoint for claude.ai (sends to /printer path)
	mcpRoute := func(c *fiber.Ctx) error {
		body := c.Body()

		resp, err := mcpHandler.HandleRequest(body)
		if err != nil {
			slog.Error("MCP handler error", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		if resp == nil {
			return c.SendStatus(fiber.StatusNoContent)
		}

		c.Set("Content-Type", "application/json")
		return c.Send(resp)
	}

	app.Post("/printer", oauth.BearerAuthMiddleware(keyPair), mcpRoute)
	app.Post("/printer/*", oauth.BearerAuthMiddleware(keyPair), mcpRoute)

	// SSE endpoint for streamable HTTP transport
	sseHandler := func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		return c.SendString("event: endpoint\ndata: /mcp\n\n")
	}
	app.Get("/mcp", oauth.BearerAuthMiddleware(keyPair), sseHandler)

	// Root path MCP handlers (Claude and other clients connect to the server URL root)
	app.Post("/", oauth.BearerAuthMiddleware(keyPair), mcpRoute)
	app.Get("/", oauth.BearerAuthMiddleware(keyPair), sseHandler)

	// Download URL for print_url tool (proxied through VPN if enabled)
	app.Get("/download", oauth.BearerAuthMiddleware(keyPair), func(c *fiber.Ctx) error {
		url := c.Query("url")
		if url == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url parameter required"})
		}

		transport := &fiber.Map{"url": url}
		_ = transport

		return c.JSON(fiber.Map{"status": "use print_url tool instead"})
	})

	slog.Info("application initialized",
		"public_url", cfg.PublicURL,
		"printer_ip", cfg.PrinterIP,
		"printer_name", cfg.PrinterName,
		"vpn_enabled", tunnel != nil,
	)

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		fiberLambda = fiberadapter.New(app)
	} else {
		slog.Info("starting local server on :3000")
		go func() {
			if err := app.Listen(":3000"); err != nil {
				slog.Error("server error", "error", err)
			}
		}()
	}
}

func handleLambdaEvent(ctx context.Context, event json.RawMessage) (interface{}, error) {
	var probe struct {
		Source     string `json:"source"`
		DetailType string `json:"detail-type"`
	}
	if err := json.Unmarshal(event, &probe); err == nil &&
		probe.Source == "aws.events" &&
		strings.Contains(probe.DetailType, "Scheduled") {
		return handleScheduledEvent(ctx)
	}

	// Alexa skill request
	var alexaProbe struct {
		Version string `json:"version"`
		Session *struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
		Request *struct {
			Type string `json:"type"`
		} `json:"request"`
	}
	if err := json.Unmarshal(event, &alexaProbe); err == nil &&
		alexaProbe.Version != "" &&
		alexaProbe.Session != nil &&
		alexaProbe.Request != nil &&
		alexaProbe.Request.Type != "" {
		return handleAlexaEvent(ctx, event)
	}

	var apiGWEvent events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(event, &apiGWEvent); err != nil {
		slog.Error("failed to parse API Gateway V2 event", "error", err)
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}
	return fiberLambda.ProxyWithContextV2(ctx, apiGWEvent)
}

func handleAlexaEvent(_ context.Context, event json.RawMessage) (interface{}, error) {
	if alexaHandler == nil {
		slog.Warn("received Alexa request but ALEXA_SKILL_ID not configured")
		return nil, fmt.Errorf("Alexa handler not configured")
	}

	slog.Info("processing Alexa skill request")
	respBytes, err := alexaHandler.HandleRequest(event)
	if err != nil {
		slog.Error("Alexa handler error", "error", err)
		return nil, err
	}

	var resp interface{}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("marshal alexa response: %w", err)
	}
	return resp, nil
}

func handleScheduledEvent(_ context.Context) (interface{}, error) {
	slog.Info("running printer keepalive")

	result := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Query printer info via IPP to keep it awake
	info, err := printerIPP.GetPrinterInfo()
	if err != nil {
		slog.Error("keepalive: IPP query failed", "error", err)
		result["ipp"] = "error"
	} else {
		slog.Info("keepalive: IPP query OK", "state", info.State, "model", info.MakeModel)
		result["ipp"] = "ok"
		result["state"] = info.State
	}

	// Also query supply levels via SNMP
	supplies, err := printerSNMP.GetSupplyLevels()
	if err != nil {
		slog.Error("keepalive: SNMP query failed", "error", err)
		result["snmp"] = "error"
	} else {
		slog.Info("keepalive: SNMP query OK", "supplies", len(supplies.Supplies))
		result["snmp"] = "ok"
	}

	result["status"] = "completed"
	return result, nil
}

func main() {
	// Suppress unused import warnings
	_ = io.Discard

	if fiberLambda != nil {
		lambda.Start(handleLambdaEvent)
	} else {
		select {}
	}
}
