package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

const defaultMobileWGTunnelName = "arc"

type mobilePayload struct {
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Username            string `json:"username"`
	PrivateKeyPEM       string `json:"privateKeyPem"`
	WireGuardConfig     string `json:"wireguardConfig,omitempty"`
	WireGuardTunnelName string `json:"wireguardTunnelName,omitempty"`
}

func buildMobilePayload(host string, wg wgConfig) (string, error) {
	if err := ensureLocalMobileSSHKeyPair(); err != nil {
		return "", err
	}

	privPath := userMobileSSHPrivateKeyPath()
	privateKey, err := os.ReadFile(privPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", privPath, err)
	}
	privateKeyPEM := strings.TrimSpace(string(privateKey))
	if privateKeyPEM == "" {
		return "", fmt.Errorf("%s is empty", privPath)
	}

	mobileWGConf := strings.TrimSpace(wg.MobileClientConf)
	if mobileWGConf == "" {
		mobileWGConf = strings.TrimSpace(wg.ClientConf)
	}

	payloadHost := strings.TrimSpace(host)
	if mobileWGConf != "" {
		payloadHost = wgServerIP
	}
	if payloadHost == "" {
		return "", fmt.Errorf("mobile payload host is empty")
	}
	if parsed := net.ParseIP(payloadHost); parsed == nil {
		payloadHost = strings.TrimSpace(strings.Trim(payloadHost, "[]"))
	}

	payload := mobilePayload{
		Host:          payloadHost,
		Port:          22,
		Username:      arcUser,
		PrivateKeyPEM: privateKeyPEM,
	}
	if mobileWGConf != "" {
		payload.WireGuardConfig = mobileWGConf
		payload.WireGuardTunnelName = defaultMobileWGTunnelName
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal mobile payload: %w", err)
	}
	return string(raw), nil
}
