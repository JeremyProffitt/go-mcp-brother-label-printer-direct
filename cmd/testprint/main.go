package main

import (
	"fmt"
	"image/png"
	"net"
	"os"
	"time"

	"go-mcp-brother-label-printer-direct/internal/label"
	"go-mcp-brother-label-printer-direct/internal/printer"
)

func main() {
	ip := "192.168.1.243"

	items := []label.KeyValue{
		{Key: "Name", Value: "John Smith"},
		{Key: "ID", Value: "12345"},
		{Key: "Dept", Value: "Engineering"},
		{Key: "Floor", Value: "3"},
		{Key: "Ext", Value: "x4521"},
		{Key: "Role", Value: "SWE"},
		{Key: "Start", Value: "2024-01"},
	}

	tapeWidth := 24.0
	fmt.Printf("Rendering table for %.0fmm tape...\n", tapeWidth)

	img := label.RenderTable(items, tapeWidth)
	fmt.Printf("  Image: %dx%d px\n", img.Bounds().Dx(), img.Bounds().Dy())

	f, _ := os.Create("label_preview.png")
	png.Encode(f, img)
	f.Close()
	fmt.Println("  Saved: label_preview.png")

	// Encode as Brother PT raster
	rasterData := label.EncodeBrotherRaster(img, tapeWidth, true)
	fmt.Printf("  Brother raster: %d bytes\n", len(rasterData))
	os.WriteFile("label_test.bin", rasterData, 0644)

	// Try 1: Send via IPP as application/octet-stream
	fmt.Println("\n=== Try 1: IPP application/octet-stream ===")
	ipp := printer.NewIPPClient(ip, nil)
	result, err := ipp.PrintDocument(rasterData, "application/octet-stream", "Table Label", 1)
	if err != nil {
		fmt.Printf("  IPP failed: %v\n", err)
	} else {
		fmt.Printf("  SUCCESS: job=%d status=%s\n", result.JobID, result.Status)
		os.Exit(0)
	}

	// Try 2: Send raw via TCP port 9100
	fmt.Println("\n=== Try 2: Raw TCP port 9100 ===")
	conn, err := net.DialTimeout("tcp", ip+":9100", 5*time.Second)
	if err != nil {
		fmt.Printf("  Port 9100 not open: %v\n", err)
	} else {
		_, err = conn.Write(rasterData)
		conn.Close()
		if err != nil {
			fmt.Printf("  Write failed: %v\n", err)
		} else {
			fmt.Println("  Sent successfully via port 9100")
			os.Exit(0)
		}
	}

	// Try 3: Send raw via TCP port 9100 with longer init sequence
	fmt.Println("\n=== Try 3: Raw TCP port 631 ===")
	conn, err = net.DialTimeout("tcp", ip+":631", 5*time.Second)
	if err != nil {
		fmt.Printf("  Connect failed: %v\n", err)
	} else {
		_, err = conn.Write(rasterData)
		conn.Close()
		if err != nil {
			fmt.Printf("  Write failed: %v\n", err)
		} else {
			fmt.Println("  Sent successfully via port 631 raw")
		}
	}
}
