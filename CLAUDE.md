# go-mcp-brother-label-printer-direct

## Overview
AWS Lambda MCP server for direct Brother PT-P750W label printer management via IPP/SNMP.
Runs serverless with OAuth 2.1 authentication and WireGuard VPN tunnel to reach the printer on a home network.

## Architecture
- **Runtime:** Go on AWS Lambda (arm64, provided.al2023)
- **Framework:** GoFiber v2 + aws-lambda-go-api-proxy
- **Printer Protocol:** IPP (Internet Printing Protocol) over HTTP, SNMP for supply levels
- **OAuth Storage:** DynamoDB single-table design with TTL
- **VPN:** WireGuard userspace (netstack) - pure Go, no kernel module
- **Observability:** OpenTelemetry + CloudWatch Logs

## Key Directories
- `cmd/lambda/` — Lambda entry point
- `internal/mcp/` — MCP JSON-RPC protocol handler and tool definitions
- `internal/alexa/` — Alexa Custom Skill handler (voice interface)
- `internal/printer/` — IPP client and SNMP queries for direct printer communication
- `internal/oauth/` — OAuth 2.1 server (authorize, token, registration, metadata)
- `internal/token/` — Ed25519 JWT signing/validation
- `internal/store/` — DynamoDB persistence for OAuth state
- `internal/vpn/` — WireGuard tunnel (userspace netstack)
- `internal/config/` — Environment-based configuration
- `internal/middleware/` — HTTP middleware (logging, auth, OTel)
- `internal/telemetry/` — OpenTelemetry setup
- `alexa/` — Alexa skill manifest and interaction model

## Printer Configuration
Configured via `PRINTER_IP` environment variable (default: 192.168.1.243).
Communicates with Brother PT-P750W label printer via:
- IPP on port 631 for label print jobs and queue management
- SNMP on port 161 for tape cartridge supply levels and status

## MCP Tools
- `get_printer_info` — Get printer model, status, capabilities, supported tape media
- `get_supply_levels` — Check tape cartridge supply levels via SNMP
- `print_label` — Print a text label (auto-sized, auto-cut)
- `print_label_image` — Download and print an image as a label
- `get_print_queue` — View current print jobs
- `get_job_status` — Check a specific job's status
- `cancel_job` — Cancel a print job
- `test_connectivity` — Test IPP, HTTP, and SNMP port reachability

## Build
```bash
make build
# or
cd cmd/lambda && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .
```

## Test
```bash
go test -v ./...
```

## Deploy
```bash
make deploy
# or via GitHub Actions on push to main
```

## Alexa Skill
Optional Alexa Custom Skill integration. Configure via `ALEXA_SKILL_ID` env var.
The Lambda detects Alexa invocations (direct Lambda calls) vs API Gateway requests automatically.
Invocation name: "my label printer" (e.g., "Alexa, ask my label printer to check supply levels").

## AWS Region
Deploy to us-east-2.
