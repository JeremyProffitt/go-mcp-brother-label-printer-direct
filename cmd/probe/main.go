package main

import (
	"fmt"
	"os"

	"go-mcp-brother-label-printer-direct/internal/printer"
)

func main() {
	ip := "192.168.1.243"
	if len(os.Args) > 1 {
		ip = os.Args[1]
	}

	fmt.Printf("=== Probing Brother PT-P750W at %s ===\n\n", ip)

	// IPP probe
	fmt.Println("--- IPP Get-Printer-Attributes ---")
	ipp := printer.NewIPPClient(ip, nil)
	info, err := ipp.GetPrinterInfo()
	if err != nil {
		fmt.Printf("IPP error: %v\n", err)
	} else {
		fmt.Printf("Name:         %s\n", info.Name)
		fmt.Printf("Make/Model:   %s\n", info.MakeModel)
		fmt.Printf("State:        %s\n", info.State)
		fmt.Printf("StateReasons: %v\n", info.StateReasons)
		fmt.Printf("Location:     %s\n", info.Location)
		fmt.Printf("Info:         %s\n", info.Info)
		if info.Capabilities != nil {
			fmt.Printf("Color:        %v\n", info.Capabilities.Color)
			fmt.Printf("Duplex:       %v\n", info.Capabilities.Duplex)
			fmt.Printf("AutoCut:      %v\n", info.Capabilities.AutoCut)
			fmt.Printf("MediaReady:   %v\n", info.Capabilities.MediaReady)
			fmt.Printf("TapeWidths:   %v\n", info.Capabilities.TapeWidths)
			fmt.Printf("MediaTypes:   %v\n", info.Capabilities.MediaTypes)
			fmt.Printf("Resolutions:  %v\n", info.Capabilities.Resolutions)
			fmt.Printf("DocFormats:   %v\n", info.Capabilities.DocumentFormats)
		}
	}

	// Raw IPP dump - get ALL attributes to see what Brother reports
	fmt.Println("\n--- IPP Raw Attribute Dump ---")
	rawInfo := printer.DumpAllAttributes(ip, nil)
	for k, v := range rawInfo {
		fmt.Printf("  %-40s = %v\n", k, v)
	}

	// SNMP probe
	fmt.Println("\n--- SNMP Supply Levels ---")
	snmp := printer.NewSNMPClient(ip, nil)
	status, err := snmp.GetSupplyLevels()
	if err != nil {
		fmt.Printf("SNMP error: %v\n", err)
	} else if status.Error != "" {
		fmt.Printf("SNMP error: %s\n", status.Error)
	} else {
		for i, s := range status.Supplies {
			fmt.Printf("  [%d] Name=%q  Level=%d  Max=%d  Color=%q  Type=%q\n",
				i, s.Name, s.Level, s.MaxLevel, s.Color, s.Type)
		}
	}

	// Connectivity
	fmt.Println("\n--- Connectivity ---")
	conn := printer.TestConnectivity(ip, nil)
	fmt.Printf("  IPP (631):  %v\n", conn.IPPReachable)
	fmt.Printf("  HTTP (80):  %v\n", conn.HTTPReachable)
	fmt.Printf("  SNMP (161): %v\n", conn.SNMPReachable)
}
