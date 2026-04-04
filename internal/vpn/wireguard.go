package vpn

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

type WireGuardConfig struct {
	PrivateKey     string `json:"private_key"`
	PeerPublicKey  string `json:"peer_public_key"`
	PeerEndpoint   string `json:"peer_endpoint"`
	PeerAllowedIPs string `json:"peer_allowed_ips"`
	Address        string `json:"address"`
	DNS            string `json:"dns,omitempty"`
}

type Tunnel struct {
	Net    *netstack.Net
	Device *device.Device
}

// LoadConfigFromEnv loads WireGuard configuration from environment variables.
func LoadConfigFromEnv() (*WireGuardConfig, error) {
	privateKey := os.Getenv("WG_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("WG_PRIVATE_KEY is not set")
	}

	cfg := &WireGuardConfig{
		PrivateKey:     privateKey,
		PeerPublicKey:  os.Getenv("WG_PEER_PUBLIC_KEY"),
		PeerEndpoint:   os.Getenv("WG_PEER_ENDPOINT"),
		PeerAllowedIPs: envOrDefault("WG_PEER_ALLOWED_IPS", "0.0.0.0/0"),
		Address:        os.Getenv("WG_ADDRESS"),
		DNS:            os.Getenv("WG_DNS"),
	}

	if cfg.PeerPublicKey == "" {
		return nil, fmt.Errorf("WG_PEER_PUBLIC_KEY is not set")
	}
	if cfg.PeerEndpoint == "" {
		return nil, fmt.Errorf("WG_PEER_ENDPOINT is not set")
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("WG_ADDRESS is not set")
	}

	return cfg, nil
}

func StartTunnel(cfg *WireGuardConfig) (*Tunnel, error) {
	localAddr, err := netip.ParsePrefix(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("parse address %q: %w", cfg.Address, err)
	}

	var dnsAddrs []netip.Addr
	if cfg.DNS != "" {
		dnsAddr, err := netip.ParseAddr(cfg.DNS)
		if err != nil {
			slog.Warn("failed to parse DNS address", "dns", cfg.DNS, "error", err)
		} else {
			dnsAddrs = append(dnsAddrs, dnsAddr)
		}
	}

	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{localAddr.Addr()},
		dnsAddrs,
		1420,
	)
	if err != nil {
		return nil, fmt.Errorf("create netstack tun: %w", err)
	}

	logger := device.NewLogger(device.LogLevelSilent, "wireguard: ")
	dev := device.NewDevice(tun, conn.NewDefaultBind(), logger)

	allowedIPLines := ""
	for _, cidr := range strings.Split(cfg.PeerAllowedIPs, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr != "" {
			allowedIPLines += fmt.Sprintf("allowed_ip=%s\n", cidr)
		}
	}

	ipcConfig := fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=%s\n%spersistent_keepalive_interval=25",
		cfg.PrivateKey,
		cfg.PeerPublicKey,
		cfg.PeerEndpoint,
		allowedIPLines,
	)

	if err := dev.IpcSet(ipcConfig); err != nil {
		dev.Close()
		return nil, fmt.Errorf("configure WireGuard device: %w", err)
	}

	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("bring up WireGuard device: %w", err)
	}

	slog.Info("WireGuard tunnel established",
		"local_addr", cfg.Address,
		"peer_endpoint", cfg.PeerEndpoint,
		"allowed_ips", cfg.PeerAllowedIPs,
	)

	return &Tunnel{Net: tnet, Device: dev}, nil
}

func (t *Tunnel) DialContext(network, addr string) (net.Conn, error) {
	return t.Net.Dial(network, addr)
}

func (t *Tunnel) Close() {
	if t.Device != nil {
		t.Device.Close()
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
