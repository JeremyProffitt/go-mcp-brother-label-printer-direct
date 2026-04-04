package config

import (
	"os"
	"testing"
)

func TestLoadRequiresAdminPassword(t *testing.T) {
	// Clear any existing env var
	os.Unsetenv("ADMIN_PASSWORD")

	_, err := Load()
	if err == nil {
		t.Error("expected error when ADMIN_PASSWORD not set")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Setenv("ADMIN_PASSWORD", "test-password")
	defer os.Unsetenv("ADMIN_PASSWORD")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PublicURL != "http://localhost:3000" {
		t.Errorf("expected default PublicURL, got %q", cfg.PublicURL)
	}
	if cfg.AdminUser != "admin" {
		t.Errorf("expected default AdminUser 'admin', got %q", cfg.AdminUser)
	}
	if cfg.PrinterIP != "192.168.1.243" {
		t.Errorf("expected default PrinterIP '192.168.1.243', got %q", cfg.PrinterIP)
	}
	if cfg.DynamoDBTable != "mcp-brother-label-printer-direct-oauth" {
		t.Errorf("expected default DynamoDBTable, got %q", cfg.DynamoDBTable)
	}
	if cfg.AccessTokenTTL.Seconds() != 3600 {
		t.Errorf("expected default AccessTokenTTL 3600s, got %v", cfg.AccessTokenTTL)
	}
}

func TestLoadCustomValues(t *testing.T) {
	os.Setenv("ADMIN_PASSWORD", "test-password")
	os.Setenv("PRINTER_IP", "10.0.0.50")
	os.Setenv("PUBLIC_URL", "https://custom.example.com")
	os.Setenv("ADMIN_USER", "custom-user")
	defer func() {
		os.Unsetenv("ADMIN_PASSWORD")
		os.Unsetenv("PRINTER_IP")
		os.Unsetenv("PUBLIC_URL")
		os.Unsetenv("ADMIN_USER")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PrinterIP != "10.0.0.50" {
		t.Errorf("expected PrinterIP '10.0.0.50', got %q", cfg.PrinterIP)
	}
	if cfg.PublicURL != "https://custom.example.com" {
		t.Errorf("expected PublicURL 'https://custom.example.com', got %q", cfg.PublicURL)
	}
	if cfg.AdminUser != "custom-user" {
		t.Errorf("expected AdminUser 'custom-user', got %q", cfg.AdminUser)
	}
}
