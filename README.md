# go-mcp-brother-label-printer-direct

A serverless MCP (Model Context Protocol) server and Alexa Custom Skill that provides direct Brother PT-P750W label printer control via AI assistants and voice. Runs on AWS Lambda with OAuth 2.1 authentication and a WireGuard VPN tunnel to reach the printer on a private home network.

## What It Does

This single Lambda function serves two interfaces to the same label printer:

- **MCP Server** for Claude (claude.ai, Claude Code, and other MCP clients) -- query printer status, check tape levels, print labels, manage print jobs
- **Alexa Custom Skill** for voice control -- "Alexa, ask my label printer to check supply levels"

Both interfaces share the same printer communication layer (IPP/SNMP), VPN tunnel, and OAuth authentication.

## Architecture

```
                          +------------------+
                          |  AWS Lambda      |
                          |  (Go, arm64)     |
  Claude / MCP Client --> |                  |
    (API Gateway HTTP)    |  Event Router    |
                          |   |              |
  Alexa Echo Device ----> |   +-> MCP Handler (JSON-RPC 2.0)
    (Direct Invoke)       |   +-> Alexa Handler (ASK JSON)
                          |   +-> Keepalive (CloudWatch Schedule)
  CloudWatch Schedule --> |   |              |
                          |  Printer Clients |
                          |   +-> IPP (631)  |-----> WireGuard -----> Brother PT-P750W
                          |   +-> SNMP (161) |       VPN Tunnel       (Home Network)
                          +------------------+
                                 |
                          +------+------+
                          |  DynamoDB   |  (OAuth state: clients,
                          |  (single    |   auth codes, refresh tokens)
                          |   table)    |
                          +-------------+
```

### Key Technologies

| Component | Technology |
|-----------|-----------|
| Runtime | Go on AWS Lambda (arm64, provided.al2023) |
| Web Framework | GoFiber v2 + aws-lambda-go-api-proxy |
| Printer Protocol | IPP (Internet Printing Protocol) over HTTP |
| Supply Monitoring | SNMP v2c (RFC 3805) |
| Authentication | OAuth 2.1 with PKCE, Ed25519 JWTs |
| Storage | DynamoDB single-table design with TTL |
| VPN | WireGuard userspace (pure Go netstack, no kernel module) |
| Observability | OpenTelemetry + CloudWatch Logs |
| Deployment | AWS SAM + GitHub Actions CI/CD |

## Available Label Printer Operations

These operations are available through both MCP tools and Alexa voice commands:

| Operation | MCP Tool | Alexa Voice Command |
|-----------|----------|-------------------|
| Printer status | `get_printer_info` | "check printer status" |
| Tape supply levels | `get_supply_levels` | "check supply levels" |
| Print text label | `print_label` | "print [text]" |
| Print image label | `print_label_image` | -- |
| Print queue | `get_print_queue` | "check the print queue" |
| Job status | `get_job_status` | "check job [number]" |
| Cancel job | `cancel_job` | "cancel job [number]" |
| Test connectivity | `test_connectivity` | "test connectivity" |

## Brother PT-P750W Specifics

- **Tape:** TZe laminated tape cartridges (3.5mm, 6mm, 9mm, 12mm, 18mm, 24mm widths)
- **Auto-cutter:** Built-in, auto-cuts after each label
- **Resolution:** 180x360 dpi or 360x720 dpi
- **Connectivity:** WiFi (this server uses IPP over the network)
- **Default IP:** 192.168.1.243

## Project Structure

```
go-mcp-brother-label-printer-direct/
  cmd/lambda/              Lambda entry point & event routing
  internal/
    alexa/                 Alexa Custom Skill handler
    mcp/                   MCP JSON-RPC protocol handler
      handler.go           Tool dispatch, resources, prompts
    printer/               Printer communication
      ipp.go               IPP binary protocol client
      snmp.go              SNMP supply level queries
      types.go             Shared data types
    oauth/                 OAuth 2.1 server
    token/                 Ed25519 JWT signing/validation
    store/                 DynamoDB persistence
    vpn/                   WireGuard userspace tunnel
    config/                Environment-based configuration
    middleware/            HTTP middleware (logging, tracing)
    telemetry/             OpenTelemetry setup
  alexa/                   Alexa skill configuration
  template.yaml            AWS SAM template
  .github/workflows/
    deploy.yml             CI/CD pipeline
```

## Setup Guide

### Prerequisites

- Go 1.25+
- AWS CLI configured with credentials
- AWS SAM CLI
- A Brother PT-P750W (or compatible IPP label printer) on your network
- (Optional) WireGuard VPN to reach a private network from Lambda

### Build

```bash
make build
# or manually:
cd cmd/lambda && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .
```

### Test

```bash
go test -v ./...
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PUBLIC_URL` | No | `http://localhost:3000` | OAuth issuer URL (your domain) |
| `ADMIN_USER` | No | `admin` | Admin username for OAuth login |
| `ADMIN_PASSWORD` | Yes | -- | Bcrypt-hashed admin password |
| `DYNAMODB_TABLE` | No | `mcp-brother-label-printer-direct-oauth` | DynamoDB table name |
| `JWT_SIGNING_KEY_ARN` | No | -- | Secrets Manager ARN for Ed25519 key pair |
| `WG_CONFIG_SECRET_ARN` | No | -- | Secrets Manager ARN for WireGuard config |
| `PRINTER_IP` | No | `192.168.1.243` | Printer IP address |
| `PRINTER_NAME` | No | `Brother PT-P750W` | Friendly printer name |
| `ALEXA_SKILL_ID` | No | -- | Alexa skill application ID (enables Alexa) |
| `OTEL_ENDPOINT` | No | `http://192.168.1.202:4318` | OpenTelemetry OTLP endpoint |

### Deploy

```bash
# Via SAM CLI
sam deploy --guided

# Or via GitHub Actions (push to main)
git push origin main
```

The GitHub Actions workflow builds the Go binary, runs tests, and deploys via SAM. Configure these GitHub repository secrets/variables:

| GitHub Secret/Variable | Type | Description |
|----------------------|------|-------------|
| `AWS_ACCESS_KEY_ID` | Secret | AWS credentials |
| `AWS_SECRET_ACCESS_KEY` | Secret | AWS credentials |
| `ADMIN_PASSWORD` | Secret | Bcrypt hash of admin password |
| `JWT_SIGNING_KEY_ARN` | Secret | Secrets Manager ARN |
| `WG_CONFIG_SECRET_ARN` | Secret | Secrets Manager ARN |
| `ADMIN_USER` | Variable | Admin username |
| `DOMAIN_NAME` | Variable | Custom domain |
| `HOSTED_ZONE_ID` | Variable | Route53 hosted zone ID |
| `CERTIFICATE_ARN_US_EAST_2` | Variable | ACM certificate ARN |
| `CLOUDFORMATION_S3_BUCKET` | Variable | S3 bucket for SAM artifacts |
| `PRINTER_IP` | Variable | Printer IP address |
| `ALEXA_SKILL_ID` | Variable | Alexa skill ID (optional) |

---

## MCP Tools

**Read-only tools:**
- `get_printer_info` -- printer model, status, capabilities, supported tape media
- `get_supply_levels` -- tape cartridge supply levels via SNMP
- `get_print_queue` -- list all active and pending label print jobs
- `get_job_status` -- check a specific job by ID
- `test_connectivity` -- test IPP, HTTP, and SNMP ports

**Destructive tools (marked with `destructiveHint`):**
- `print_label` -- print a text label (auto-sized, auto-cut)
- `print_label_image` -- download and print an image as a label
- `cancel_job` -- cancel a print job by ID

## MCP Resources

- `printer://info` -- current printer status (JSON)
- `printer://supplies` -- current tape cartridge levels (JSON)
- `printer://help` -- help guide (Markdown)

## MCP Prompts

- `diagnose-printer` -- run a full diagnostic (connectivity, status, tape levels, queue)
- `supply-check` -- check tape supplies and flag low/critical levels
- `print-label` -- print a label with smart defaults

---

## Printer Communication

### IPP (Internet Printing Protocol)

The server implements IPP operations directly in Go (no external libraries), communicating with the printer on port 631:

- `Print-Job` (0x0002) -- submit label print jobs
- `Cancel-Job` (0x0008) -- cancel print jobs
- `Get-Job-Attributes` (0x0009) -- query job status
- `Get-Jobs` (0x000A) -- list print queue
- `Get-Printer-Attributes` (0x000B) -- query printer info and capabilities

### SNMP (Simple Network Management Protocol)

Supply levels are queried via SNMP v2c on port 161 using RFC 3805 Printer MIB OIDs:

| OID | Data |
|-----|------|
| `1.3.6.1.2.1.43.11.1.1.6.1.*` | Supply description (tape cartridge name) |
| `1.3.6.1.2.1.43.11.1.1.8.1.*` | Max capacity |
| `1.3.6.1.2.1.43.11.1.1.9.1.*` | Current level |
| `1.3.6.1.2.1.43.12.1.1.4.1.*` | Colorant value |
