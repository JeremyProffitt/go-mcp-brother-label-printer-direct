package printer

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/gosnmp/gosnmp"
)

// SNMP OIDs for printer supply levels (RFC 3805)
const (
	// prtMarkerSuppliesDescription
	OIDSupplyDescription = "1.3.6.1.2.1.43.11.1.1.6.1"
	// prtMarkerSuppliesMaxCapacity
	OIDSupplyMaxCapacity = "1.3.6.1.2.1.43.11.1.1.8.1"
	// prtMarkerSuppliesLevel
	OIDSupplyCurrentLevel = "1.3.6.1.2.1.43.11.1.1.9.1"
	// prtMarkerSuppliesType
	OIDSupplyType = "1.3.6.1.2.1.43.11.1.1.5.1"
	// prtMarkerColorantValue (color name)
	OIDColorantValue = "1.3.6.1.2.1.43.12.1.1.4.1"
)

// SNMPClient queries printer SNMP for supply levels.
type SNMPClient struct {
	printerIP string
	dialFunc  func(network, addr string) (net.Conn, error)
}

// NewSNMPClient creates a new SNMP client.
func NewSNMPClient(printerIP string, dialFunc func(network, addr string) (net.Conn, error)) *SNMPClient {
	return &SNMPClient{
		printerIP: printerIP,
		dialFunc:  dialFunc,
	}
}

// GetSupplyLevels queries SNMP for ink/toner supply levels.
func (s *SNMPClient) GetSupplyLevels() (*SupplyStatus, error) {
	status := &SupplyStatus{
		PrinterName: s.printerIP,
	}

	snmpClient := &gosnmp.GoSNMP{
		Target:    s.printerIP,
		Port:      161,
		Version:   gosnmp.Version2c,
		Community: "public",
		Timeout:   5 * time.Second,
		Retries:   1,
	}

	if s.dialFunc != nil {
		snmpClient.Transport = "udp"
	}

	err := snmpClient.Connect()
	if err != nil {
		status.Error = fmt.Sprintf("SNMP connect failed: %v", err)
		return status, nil
	}
	defer snmpClient.Conn.Close()

	// Get supply descriptions
	descriptions, err := s.walkOID(snmpClient, OIDSupplyDescription)
	if err != nil {
		slog.Warn("SNMP walk supply descriptions failed", "error", err)
		status.Error = fmt.Sprintf("SNMP walk failed: %v", err)
		return status, nil
	}

	// Get max capacity
	maxCapacities, err := s.walkOID(snmpClient, OIDSupplyMaxCapacity)
	if err != nil {
		slog.Warn("SNMP walk max capacity failed", "error", err)
	}

	// Get current levels
	currentLevels, err := s.walkOID(snmpClient, OIDSupplyCurrentLevel)
	if err != nil {
		slog.Warn("SNMP walk current levels failed", "error", err)
	}

	// Get colorant values (colors)
	colorants, err := s.walkOID(snmpClient, OIDColorantValue)
	if err != nil {
		slog.Debug("SNMP walk colorants failed (optional)", "error", err)
	}

	status.SNMPSuccess = true

	for i, desc := range descriptions {
		supply := SupplyLevel{
			Name: desc.Value,
		}

		if i < len(maxCapacities) {
			supply.MaxLevel = maxCapacities[i].IntValue
		}
		if i < len(currentLevels) {
			currLevel := currentLevels[i].IntValue
			supply.Level = currLevel

			// Calculate percentage if max > 0
			if supply.MaxLevel > 0 && currLevel >= 0 {
				supply.Level = (currLevel * 100) / supply.MaxLevel
			}

			// -3 means "unknown" in SNMP supply levels
			if currLevel == -3 {
				supply.Level = -1
			}
			// -2 means "unknown" (vendor-specific)
			if currLevel == -2 {
				supply.Level = -1
			}
		}

		if i < len(colorants) {
			supply.Color = colorants[i].Value
		}

		// Determine type from name
		supply.Type = guessSupplyType(supply.Name)

		status.Supplies = append(status.Supplies, supply)
	}

	slog.Info("SNMP supply levels retrieved", "printer", s.printerIP, "supplies", len(status.Supplies))
	return status, nil
}

type snmpValue struct {
	Value    string
	IntValue int
}

func (s *SNMPClient) walkOID(client *gosnmp.GoSNMP, oid string) ([]snmpValue, error) {
	var results []snmpValue

	err := client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		sv := snmpValue{}
		switch pdu.Type {
		case gosnmp.OctetString:
			sv.Value = string(pdu.Value.([]byte))
		case gosnmp.Integer:
			sv.IntValue = pdu.Value.(int)
			sv.Value = fmt.Sprintf("%d", pdu.Value.(int))
		case gosnmp.Gauge32:
			val := pdu.Value.(uint)
			sv.IntValue = int(val)
			sv.Value = fmt.Sprintf("%d", val)
		default:
			sv.Value = fmt.Sprintf("%v", pdu.Value)
		}
		results = append(results, sv)
		return nil
	})

	return results, err
}

func guessSupplyType(name string) string {
	lower := fmt.Sprintf("%s", name)
	switch {
	case contains(lower, "toner"):
		return "toner"
	case contains(lower, "ink"):
		return "ink"
	case contains(lower, "drum"):
		return "drum"
	case contains(lower, "fuser"):
		return "fuser"
	case contains(lower, "waste"):
		return "waste"
	case contains(lower, "transfer"):
		return "transfer"
	default:
		return "supply"
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c := s[i+j]
			// Case-insensitive
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			sc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if c != sc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
