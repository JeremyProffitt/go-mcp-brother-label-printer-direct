package printer

import (
	"encoding/binary"
	"testing"
)

func TestBuildGetPrinterAttrsRequest(t *testing.T) {
	data := buildGetPrinterAttrsRequest("http://192.168.1.118:631/ipp/print", []string{"printer-name"})

	if len(data) < 8 {
		t.Fatal("request too short")
	}

	// Check version
	if data[0] != 0x01 || data[1] != 0x01 {
		t.Errorf("expected IPP 1.1 version, got %d.%d", data[0], data[1])
	}

	// Check operation code
	opCode := binary.BigEndian.Uint16(data[2:4])
	if opCode != OpGetPrinterAttributes {
		t.Errorf("expected Get-Printer-Attributes (0x%04x), got 0x%04x", OpGetPrinterAttributes, opCode)
	}
}

func TestBuildPrintJobRequest(t *testing.T) {
	data := buildPrintJobRequest("http://192.168.1.118:631/ipp/print", "Test Job", "text/plain", 1)

	if len(data) < 8 {
		t.Fatal("request too short")
	}

	opCode := binary.BigEndian.Uint16(data[2:4])
	if opCode != OpPrintJob {
		t.Errorf("expected Print-Job (0x%04x), got 0x%04x", OpPrintJob, opCode)
	}
}

func TestBuildGetJobsRequest(t *testing.T) {
	data := buildGetJobsRequest("http://192.168.1.118:631/ipp/print")

	if len(data) < 8 {
		t.Fatal("request too short")
	}

	opCode := binary.BigEndian.Uint16(data[2:4])
	if opCode != OpGetJobs {
		t.Errorf("expected Get-Jobs (0x%04x), got 0x%04x", OpGetJobs, opCode)
	}
}

func TestBuildCancelJobRequest(t *testing.T) {
	data := buildCancelJobRequest("http://192.168.1.118:631/ipp/print", 42)

	if len(data) < 8 {
		t.Fatal("request too short")
	}

	opCode := binary.BigEndian.Uint16(data[2:4])
	if opCode != OpCancelJob {
		t.Errorf("expected Cancel-Job (0x%04x), got 0x%04x", OpCancelJob, opCode)
	}
}

func TestParseIPPResponseEmpty(t *testing.T) {
	attrs := parseIPPResponse([]byte{})
	if len(attrs) != 0 {
		t.Errorf("expected empty attrs for empty response, got %d", len(attrs))
	}
}

func TestParseIPPResponseMinimal(t *testing.T) {
	// Build a minimal valid IPP response:
	// version 1.1, status OK, request-id 1, operation-attrs group, charset, end tag
	var data []byte
	data = append(data, 0x01, 0x01)              // version
	data = append(data, 0x00, 0x00)              // status OK
	data = appendUint32(data, 1)                  // request-id
	data = append(data, TagOperationAttrs)        // operation attributes group
	data = appendAttribute(data, TagCharset, "attributes-charset", "utf-8")
	data = append(data, TagEnd) // end

	attrs := parseIPPResponse(data)
	if v, ok := attrs["attributes-charset"]; !ok || v[0] != "utf-8" {
		t.Errorf("expected charset 'utf-8', got %v", attrs["attributes-charset"])
	}
}

func TestParseIPPResponseInteger(t *testing.T) {
	var data []byte
	data = append(data, 0x01, 0x01)
	data = append(data, 0x00, 0x00)
	data = appendUint32(data, 1)
	data = append(data, TagPrinterAttrs)
	data = appendIntAttribute(data, TagEnum, "printer-state", 3) // idle
	data = append(data, TagEnd)

	attrs := parseIPPResponse(data)
	if v, ok := attrs["printer-state"]; !ok || v[0] != "3" {
		t.Errorf("expected printer-state '3', got %v", attrs["printer-state"])
	}
}

func TestParseIPPResponseBoolean(t *testing.T) {
	var data []byte
	data = append(data, 0x01, 0x01)
	data = append(data, 0x00, 0x00)
	data = appendUint32(data, 1)
	data = append(data, TagPrinterAttrs)
	// Boolean attribute: color-supported = true
	data = append(data, TagBoolean)
	data = appendUint16(data, uint16(len("color-supported")))
	data = append(data, "color-supported"...)
	data = appendUint16(data, 1)
	data = append(data, 0x01) // true
	data = append(data, TagEnd)

	attrs := parseIPPResponse(data)
	if v, ok := attrs["color-supported"]; !ok || v[0] != "true" {
		t.Errorf("expected color-supported 'true', got %v", attrs["color-supported"])
	}
}

func TestMapPrinterState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3", "idle"},
		{"4", "processing"},
		{"5", "stopped"},
		{"99", "unknown (99)"},
	}

	for _, tt := range tests {
		got := mapPrinterState(tt.input)
		if got != tt.expected {
			t.Errorf("mapPrinterState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMapJobState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3", "pending"},
		{"5", "processing"},
		{"7", "canceled"},
		{"9", "completed"},
		{"99", "unknown (99)"},
	}

	for _, tt := range tests {
		got := mapJobState(tt.input)
		if got != tt.expected {
			t.Errorf("mapJobState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Test helpers ---

func appendUint16(data []byte, v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return append(data, b...)
}

func appendUint32(data []byte, v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return append(data, b...)
}

func appendAttribute(data []byte, tag byte, name, value string) []byte {
	data = append(data, tag)
	data = appendUint16(data, uint16(len(name)))
	data = append(data, name...)
	data = appendUint16(data, uint16(len(value)))
	data = append(data, value...)
	return data
}

func appendIntAttribute(data []byte, tag byte, name string, value int32) []byte {
	data = append(data, tag)
	data = appendUint16(data, uint16(len(name)))
	data = append(data, name...)
	data = appendUint16(data, 4)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(value))
	return append(data, b...)
}
