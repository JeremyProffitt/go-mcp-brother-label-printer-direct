package printer

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// IPP operation codes
const (
	OpPrintJob             = 0x0002
	OpValidateJob          = 0x0004
	OpCancelJob            = 0x0008
	OpGetJobAttributes     = 0x0009
	OpGetJobs              = 0x000A
	OpGetPrinterAttributes = 0x000B
)

// IPP attribute tags
const (
	TagEnd             = 0x03
	TagOperationAttrs  = 0x01
	TagJobAttrs        = 0x02
	TagPrinterAttrs    = 0x04
	TagUnsupported     = 0x05

	// Value tags
	TagInteger         = 0x21
	TagBoolean         = 0x22
	TagEnum            = 0x23
	TagOctetString     = 0x30
	TagDateTime        = 0x31
	TagTextWithLang    = 0x35
	TagNameWithLang    = 0x36
	TagTextWithoutLang = 0x41
	TagNameWithoutLang = 0x42
	TagKeyword         = 0x44
	TagURI             = 0x45
	TagCharset         = 0x47
	TagNaturalLang     = 0x48
	TagMimeMediaType   = 0x49
)

// IPP status codes
const (
	StatusOk                   = 0x0000
	StatusOkIgnoredOrSubstituted = 0x0001
	StatusClientBadRequest     = 0x0400
	StatusClientForbidden      = 0x0401
	StatusClientNotFound       = 0x0406
	StatusServerInternalError  = 0x0500
)

// IPPClient communicates with a printer via IPP.
type IPPClient struct {
	printerIP  string
	httpClient *http.Client
}

// NewIPPClient creates a new IPP client.
// dialFunc is optional; if provided, it is used for custom network routing (e.g., VPN).
func NewIPPClient(printerIP string, dialFunc func(network, addr string) (net.Conn, error)) *IPPClient {
	transport := &http.Transport{
		MaxIdleConns:    5,
		IdleConnTimeout: 30 * time.Second,
	}
	if dialFunc != nil {
		transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return dialFunc(network, addr)
		}
	}

	return &IPPClient{
		printerIP: printerIP,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}
}

// ippURL returns the IPP endpoint URL.
func (c *IPPClient) ippURL() string {
	return fmt.Sprintf("http://%s:631/ipp/print", c.printerIP)
}

// GetPrinterInfo retrieves printer attributes via IPP Get-Printer-Attributes.
func (c *IPPClient) GetPrinterInfo() (*PrinterInfo, error) {
	reqAttrs := []string{
		"printer-name", "printer-make-and-model", "printer-state",
		"printer-state-reasons", "printer-location", "printer-info",
		"printer-uri-supported", "color-supported", "sides-supported",
		"media-supported", "media-ready", "media-type-supported",
		"printer-resolution-supported", "document-format-supported",
	}

	body := buildGetPrinterAttrsRequest(c.ippURL(), reqAttrs)
	resp, err := c.doIPP(body)
	if err != nil {
		return nil, fmt.Errorf("get printer attributes: %w", err)
	}

	attrs := parseIPPResponse(resp)
	info := &PrinterInfo{
		IP:   c.printerIP,
		URIs: make(map[string]string),
	}

	if v, ok := attrs["printer-name"]; ok {
		info.Name = v[0]
	}
	if v, ok := attrs["printer-make-and-model"]; ok {
		info.MakeModel = v[0]
	}
	if v, ok := attrs["printer-state"]; ok {
		info.State = mapPrinterState(v[0])
	}
	if v, ok := attrs["printer-state-reasons"]; ok {
		info.StateReasons = v
	}
	if v, ok := attrs["printer-location"]; ok {
		info.Location = v[0]
	}
	if v, ok := attrs["printer-info"]; ok {
		info.Info = v[0]
	}
	if v, ok := attrs["printer-uri-supported"]; ok {
		for _, uri := range v {
			if strings.Contains(uri, "ipp") {
				info.URIs["ipp"] = uri
			} else if strings.Contains(uri, "http") {
				info.URIs["http"] = uri
			}
		}
	}

	caps := &Capabilities{
		AutoCut: true, // PT-P750W has built-in auto-cutter
	}
	if v, ok := attrs["color-supported"]; ok {
		caps.Color = v[0] == "true"
	}
	if v, ok := attrs["sides-supported"]; ok {
		caps.Duplex = len(v) > 1
	}
	if v, ok := attrs["media-ready"]; ok {
		caps.MediaReady = v
	}
	if v, ok := attrs["media-supported"]; ok {
		caps.TapeWidths = v
	}
	if v, ok := attrs["media-type-supported"]; ok {
		caps.MediaTypes = v
	}
	if v, ok := attrs["printer-resolution-supported"]; ok {
		caps.Resolutions = v
	}
	if v, ok := attrs["document-format-supported"]; ok {
		caps.DocumentFormats = v
	}
	info.Capabilities = caps

	return info, nil
}

// PrintText sends text content to the printer.
func (c *IPPClient) PrintText(text, jobName string, copies int) (*PrintResult, error) {
	if jobName == "" {
		jobName = "MCP Print Job"
	}
	if copies < 1 {
		copies = 1
	}

	body := buildPrintJobRequest(c.ippURL(), jobName, "text/plain", copies)
	body = append(body, []byte(text)...)

	resp, err := c.doIPP(body)
	if err != nil {
		return nil, fmt.Errorf("print text: %w", err)
	}

	attrs := parseIPPResponse(resp)
	result := &PrintResult{Status: "submitted"}

	if v, ok := attrs["job-id"]; ok {
		fmt.Sscanf(v[0], "%d", &result.JobID)
	}
	if v, ok := attrs["job-state"]; ok {
		result.Status = mapJobState(v[0])
	}
	result.Message = fmt.Sprintf("Job %d submitted to %s", result.JobID, c.printerIP)

	return result, nil
}

// PrintDocument sends document data to the printer.
func (c *IPPClient) PrintDocument(data []byte, mimeType, jobName string, copies int) (*PrintResult, error) {
	if jobName == "" {
		jobName = "MCP Print Job"
	}
	if copies < 1 {
		copies = 1
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	body := buildPrintJobRequest(c.ippURL(), jobName, mimeType, copies)
	body = append(body, data...)

	resp, err := c.doIPP(body)
	if err != nil {
		return nil, fmt.Errorf("print document: %w", err)
	}

	attrs := parseIPPResponse(resp)
	result := &PrintResult{Status: "submitted"}

	if v, ok := attrs["job-id"]; ok {
		fmt.Sscanf(v[0], "%d", &result.JobID)
	}
	if v, ok := attrs["job-state"]; ok {
		result.Status = mapJobState(v[0])
	}
	result.Message = fmt.Sprintf("Job %d submitted to %s", result.JobID, c.printerIP)

	return result, nil
}

// GetJobs retrieves the print queue.
func (c *IPPClient) GetJobs() ([]PrintJob, error) {
	body := buildGetJobsRequest(c.ippURL())
	resp, err := c.doIPP(body)
	if err != nil {
		return nil, fmt.Errorf("get jobs: %w", err)
	}

	return parseJobsResponse(resp), nil
}

// GetJobStatus retrieves status for a specific job.
func (c *IPPClient) GetJobStatus(jobID int) (*PrintJob, error) {
	body := buildGetJobAttrsRequest(c.ippURL(), jobID)
	resp, err := c.doIPP(body)
	if err != nil {
		return nil, fmt.Errorf("get job status: %w", err)
	}

	attrs := parseIPPResponse(resp)
	job := &PrintJob{ID: jobID}

	if v, ok := attrs["job-name"]; ok {
		job.Name = v[0]
	}
	if v, ok := attrs["job-state"]; ok {
		job.State = mapJobState(v[0])
	}
	if v, ok := attrs["job-originating-user-name"]; ok {
		job.Owner = v[0]
	}

	return job, nil
}

// CancelJob cancels a print job.
func (c *IPPClient) CancelJob(jobID int) error {
	body := buildCancelJobRequest(c.ippURL(), jobID)
	_, err := c.doIPP(body)
	if err != nil {
		return fmt.Errorf("cancel job %d: %w", jobID, err)
	}
	return nil
}

// doIPP sends an IPP request and returns the response body.
func (c *IPPClient) doIPP(body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.ippURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/ipp")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if len(respBody) < 4 {
		return nil, fmt.Errorf("IPP response too short (%d bytes)", len(respBody))
	}

	statusCode := binary.BigEndian.Uint16(respBody[2:4])
	if statusCode >= 0x0400 {
		return nil, fmt.Errorf("IPP error: status 0x%04x", statusCode)
	}

	return respBody, nil
}

// TestConnectivity tests printer reachability on multiple ports.
func TestConnectivity(printerIP string, dialFunc func(network, addr string) (net.Conn, error)) *ConnectivityResult {
	result := &ConnectivityResult{PrinterIP: printerIP}

	dial := func(network, addr string) (net.Conn, error) {
		if dialFunc != nil {
			return dialFunc(network, addr)
		}
		return net.DialTimeout(network, addr, 5*time.Second)
	}

	// Test IPP (631)
	if conn, err := dial("tcp", printerIP+":631"); err == nil {
		conn.Close()
		result.IPPReachable = true
	}

	// Test HTTP (80)
	if conn, err := dial("tcp", printerIP+":80"); err == nil {
		conn.Close()
		result.HTTPReachable = true
	}

	// Test SNMP (161 UDP)
	if conn, err := dial("udp", printerIP+":161"); err == nil {
		conn.Close()
		result.SNMPReachable = true
	}

	slog.Info("connectivity test completed",
		"printer_ip", printerIP,
		"ipp", result.IPPReachable,
		"http", result.HTTPReachable,
		"snmp", result.SNMPReachable,
	)

	return result
}

// --- IPP request builders ---

var requestID uint32 = 1

func nextRequestID() uint32 {
	requestID++
	return requestID
}

// DumpAllAttributes queries the printer for ALL attributes (no filter) and returns the raw map.
func DumpAllAttributes(printerIP string, dialFunc func(network, addr string) (net.Conn, error)) map[string][]string {
	client := NewIPPClient(printerIP, dialFunc)
	printerURI := client.ippURL()

	// Build request with no requested-attributes filter — printer returns everything
	var buf bytes.Buffer
	buf.Write([]byte{0x01, 0x01})
	binary.Write(&buf, binary.BigEndian, uint16(OpGetPrinterAttributes))
	binary.Write(&buf, binary.BigEndian, nextRequestID())
	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)
	buf.WriteByte(TagEnd)

	resp, err := client.doIPP(buf.Bytes())
	if err != nil {
		return map[string][]string{"error": {err.Error()}}
	}
	return parseIPPResponse(resp)
}

func buildGetPrinterAttrsRequest(printerURI string, attributes []string) []byte {
	var buf bytes.Buffer

	// Version 1.1
	buf.Write([]byte{0x01, 0x01})
	// Operation: Get-Printer-Attributes
	binary.Write(&buf, binary.BigEndian, uint16(OpGetPrinterAttributes))
	// Request ID
	binary.Write(&buf, binary.BigEndian, nextRequestID())

	// Operation attributes group
	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)

	// Requested attributes
	for i, attr := range attributes {
		if i == 0 {
			writeAttribute(&buf, TagKeyword, "requested-attributes", attr)
		} else {
			writeAdditionalValue(&buf, TagKeyword, attr)
		}
	}

	// End of attributes
	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func buildPrintJobRequest(printerURI, jobName, mimeType string, copies int) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0x01, 0x01})
	binary.Write(&buf, binary.BigEndian, uint16(OpPrintJob))
	binary.Write(&buf, binary.BigEndian, nextRequestID())

	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)
	writeAttribute(&buf, TagNameWithoutLang, "job-name", jobName)
	writeAttribute(&buf, TagMimeMediaType, "document-format", mimeType)

	// Job attributes
	buf.WriteByte(TagJobAttrs)
	writeIntAttribute(&buf, TagInteger, "copies", copies)

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func buildGetJobsRequest(printerURI string) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0x01, 0x01})
	binary.Write(&buf, binary.BigEndian, uint16(OpGetJobs))
	binary.Write(&buf, binary.BigEndian, nextRequestID())

	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)
	writeAttribute(&buf, TagKeyword, "which-jobs", "not-completed")

	attrs := []string{"job-id", "job-name", "job-state", "job-originating-user-name", "job-k-octets", "job-impressions"}
	for i, attr := range attrs {
		if i == 0 {
			writeAttribute(&buf, TagKeyword, "requested-attributes", attr)
		} else {
			writeAdditionalValue(&buf, TagKeyword, attr)
		}
	}

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func buildGetJobAttrsRequest(printerURI string, jobID int) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0x01, 0x01})
	binary.Write(&buf, binary.BigEndian, uint16(OpGetJobAttributes))
	binary.Write(&buf, binary.BigEndian, nextRequestID())

	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)
	writeIntAttribute(&buf, TagInteger, "job-id", jobID)

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

func buildCancelJobRequest(printerURI string, jobID int) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0x01, 0x01})
	binary.Write(&buf, binary.BigEndian, uint16(OpCancelJob))
	binary.Write(&buf, binary.BigEndian, nextRequestID())

	buf.WriteByte(TagOperationAttrs)
	writeAttribute(&buf, TagCharset, "attributes-charset", "utf-8")
	writeAttribute(&buf, TagNaturalLang, "attributes-natural-language", "en-us")
	writeAttribute(&buf, TagURI, "printer-uri", printerURI)
	writeIntAttribute(&buf, TagInteger, "job-id", jobID)

	buf.WriteByte(TagEnd)

	return buf.Bytes()
}

// --- IPP encoding helpers ---

func writeAttribute(buf *bytes.Buffer, tag byte, name, value string) {
	buf.WriteByte(tag)
	binary.Write(buf, binary.BigEndian, uint16(len(name)))
	buf.WriteString(name)
	binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
}

func writeAdditionalValue(buf *bytes.Buffer, tag byte, value string) {
	buf.WriteByte(tag)
	binary.Write(buf, binary.BigEndian, uint16(0)) // empty name = additional value
	binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
}

func writeIntAttribute(buf *bytes.Buffer, tag byte, name string, value int) {
	buf.WriteByte(tag)
	binary.Write(buf, binary.BigEndian, uint16(len(name)))
	buf.WriteString(name)
	binary.Write(buf, binary.BigEndian, uint16(4))
	binary.Write(buf, binary.BigEndian, int32(value))
}

// --- IPP response parsing ---

func parseIPPResponse(data []byte) map[string][]string {
	attrs := make(map[string][]string)
	if len(data) < 8 {
		return attrs
	}

	pos := 8 // skip version(2) + status(2) + request-id(4)
	currentName := ""

	for pos < len(data) {
		tag := data[pos]
		pos++

		// Group delimiter tags
		if tag <= 0x05 {
			if tag == TagEnd {
				break
			}
			continue
		}

		if pos+2 > len(data) {
			break
		}

		// Read name length
		nameLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+nameLen > len(data) {
			break
		}

		name := string(data[pos : pos+nameLen])
		pos += nameLen

		if pos+2 > len(data) {
			break
		}

		// Read value length
		valueLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+valueLen > len(data) {
			break
		}

		var value string
		switch {
		case tag == TagInteger || tag == TagEnum:
			if valueLen == 4 {
				intVal := int32(binary.BigEndian.Uint32(data[pos : pos+4]))
				value = fmt.Sprintf("%d", intVal)
			}
		case tag == TagBoolean:
			if valueLen >= 1 {
				if data[pos] != 0 {
					value = "true"
				} else {
					value = "false"
				}
			}
		default:
			value = string(data[pos : pos+valueLen])
		}

		pos += valueLen

		if nameLen > 0 {
			currentName = name
			attrs[currentName] = append(attrs[currentName], value)
		} else if currentName != "" {
			// Additional value for current attribute
			attrs[currentName] = append(attrs[currentName], value)
		}
	}

	return attrs
}

func parseJobsResponse(data []byte) []PrintJob {
	var jobs []PrintJob
	if len(data) < 8 {
		return jobs
	}

	pos := 8
	var current *PrintJob

	for pos < len(data) {
		tag := data[pos]
		pos++

		if tag == TagEnd {
			break
		}

		// Job attributes group delimiter = new job
		if tag == TagJobAttrs {
			if current != nil {
				jobs = append(jobs, *current)
			}
			current = &PrintJob{}
			continue
		}

		// Other group delimiters
		if tag <= 0x05 {
			continue
		}

		if pos+2 > len(data) {
			break
		}

		nameLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+nameLen > len(data) {
			break
		}
		name := string(data[pos : pos+nameLen])
		pos += nameLen

		if pos+2 > len(data) {
			break
		}
		valueLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2

		if pos+valueLen > len(data) {
			break
		}

		var value string
		switch {
		case tag == TagInteger || tag == TagEnum:
			if valueLen == 4 {
				intVal := int32(binary.BigEndian.Uint32(data[pos : pos+4]))
				value = fmt.Sprintf("%d", intVal)
			}
		default:
			value = string(data[pos : pos+valueLen])
		}
		pos += valueLen

		if current == nil {
			current = &PrintJob{}
		}

		switch name {
		case "job-id":
			fmt.Sscanf(value, "%d", &current.ID)
		case "job-name":
			current.Name = value
		case "job-state":
			current.State = mapJobState(value)
		case "job-originating-user-name":
			current.Owner = value
		case "job-k-octets":
			fmt.Sscanf(value, "%d", &current.Size)
			current.Size *= 1024
		case "job-impressions":
			fmt.Sscanf(value, "%d", &current.Pages)
		}
	}

	if current != nil && current.ID > 0 {
		jobs = append(jobs, *current)
	}

	return jobs
}

func mapPrinterState(state string) string {
	switch state {
	case "3":
		return "idle"
	case "4":
		return "processing"
	case "5":
		return "stopped"
	default:
		return "unknown (" + state + ")"
	}
}

func mapJobState(state string) string {
	switch state {
	case "3":
		return "pending"
	case "4":
		return "pending-held"
	case "5":
		return "processing"
	case "6":
		return "processing-stopped"
	case "7":
		return "canceled"
	case "8":
		return "aborted"
	case "9":
		return "completed"
	default:
		return "unknown (" + state + ")"
	}
}
