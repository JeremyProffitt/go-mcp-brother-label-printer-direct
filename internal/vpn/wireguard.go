package vpn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

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

func LoadConfig(ctx context.Context, smClient *secretsmanager.Client, secretARN string) (*WireGuardConfig, error) {
	if secretARN == "" {
		return nil, fmt.Errorf("WG_CONFIG_SECRET_ARN is not set")
	}

	result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretARN),
	})
	if err != nil {
		return nil, fmt.Errorf("get WireGuard secret: %w", err)
	}

	var cfg WireGuardConfig
	if err := json.Unmarshal([]byte(*result.SecretString), &cfg); err != nil {
		return nil, fmt.Errorf("parse WireGuard config: %w", err)
	}

	return &cfg, nil
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
