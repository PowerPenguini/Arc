package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/curve25519"
)

const (
	wgInterface = "wg0"
	wgPort      = 51820

	wgServerCIDR = "10.0.0.1/32"
	wgClientCIDR = "10.0.0.2/32"
	wgServerIP   = "10.0.0.1"
)

type wgConfig struct {
	ServerPriv string
	ServerPub  string
	ClientPriv string
	ClientPub  string

	ServerConf string
	ClientConf string
	Endpoint   string
}

func buildWGConfig(endpointHost string) (wgConfig, error) {
	host := strings.TrimSpace(endpointHost)
	if host == "" {
		return wgConfig{}, fmt.Errorf("missing host for WireGuard endpoint")
	}

	sPriv, sPub, err := genWGKeyPair()
	if err != nil {
		return wgConfig{}, err
	}
	cPriv, cPub, err := genWGKeyPair()
	if err != nil {
		return wgConfig{}, err
	}

	endpoint := fmt.Sprintf("%s:%d", host, wgPort)
	serverConf := strings.Join([]string{
		"[Interface]",
		"Address = " + wgServerCIDR,
		fmt.Sprintf("ListenPort = %d", wgPort),
		"PrivateKey = " + sPriv,
		"",
		"[Peer]",
		"PublicKey = " + cPub,
		"AllowedIPs = " + strings.Split(wgClientCIDR, "/")[0] + "/32",
		"",
	}, "\n")

	clientConf := strings.Join([]string{
		"[Interface]",
		"Address = " + wgClientCIDR,
		"PrivateKey = " + cPriv,
		"",
		"[Peer]",
		"PublicKey = " + sPub,
		"Endpoint = " + endpoint,
		"AllowedIPs = " + wgServerIP + "/32",
		"PersistentKeepalive = 25",
		"",
	}, "\n")

	return wgConfig{
		ServerPriv: sPriv,
		ServerPub:  sPub,
		ClientPriv: cPriv,
		ClientPub:  cPub,
		ServerConf: serverConf,
		ClientConf: clientConf,
		Endpoint:   endpoint,
	}, nil
}

func genWGKeyPair() (privB64, pubB64 string, err error) {
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return "", "", err
	}
	// Clamp (per X25519).
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(priv[:]), base64.StdEncoding.EncodeToString(pub), nil
}

func wgPublicKeyFromPrivateKeyB64(privB64 string) (string, error) {
	privRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privB64))
	if err != nil {
		return "", fmt.Errorf("decode wg private key: %w", err)
	}
	if len(privRaw) != 32 {
		return "", fmt.Errorf("wg private key must be 32 bytes, got %d", len(privRaw))
	}

	// Clamp (per X25519). Some inputs are already clamped, but do it defensively.
	var priv [32]byte
	copy(priv[:], privRaw)
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive wg public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

func parseWGPrivateKeyFromConf(conf string) (string, error) {
	section := ""
	for _, ln := range strings.Split(conf, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, ";") {
			continue
		}
		if strings.HasPrefix(ln, "[") && strings.HasSuffix(ln, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(ln, "["), "]")
			section = strings.TrimSpace(section)
			continue
		}
		if !strings.EqualFold(section, "Interface") {
			continue
		}
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if strings.EqualFold(k, "PrivateKey") && v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("wg conf missing [Interface] PrivateKey")
}

func patchWGPeerInConf(conf string, matchAllowedIPs string, setPeerPublicKey string, ensureEndpoint string, ensureKeepalive string) (string, bool, error) {
	lines := strings.Split(conf, "\n")

	allowedContains := func(allowedRaw, want string) bool {
		want = strings.TrimSpace(want)
		if want == "" {
			return false
		}
		// AllowedIPs may be a comma-separated list.
		for _, part := range strings.Split(allowedRaw, ",") {
			if strings.TrimSpace(part) == want {
				return true
			}
		}
		return false
	}

	type peerBlock struct {
		startIdx int
		endIdx   int // exclusive
		pubIdx   int
		allowIdx int
		endpIdx  int
		keepIdx  int
		allowed  string
	}

	section := ""
	var blocks []peerBlock
	var cur *peerBlock

	flushCur := func(end int) {
		if cur == nil {
			return
		}
		cur.endIdx = end
		blocks = append(blocks, *cur)
		cur = nil
	}

	for i := 0; i <= len(lines); i++ {
		ln := ""
		if i < len(lines) {
			ln = strings.TrimSpace(lines[i])
		}

		if i == len(lines) {
			flushCur(i)
			break
		}

		if strings.HasPrefix(ln, "[") && strings.HasSuffix(ln, "]") {
			flushCur(i)
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(ln, "["), "]"))
			if strings.EqualFold(section, "Peer") {
				cur = &peerBlock{
					startIdx: i,
					endIdx:   -1,
					pubIdx:   -1,
					allowIdx: -1,
					endpIdx:  -1,
					keepIdx:  -1,
				}
			}
			continue
		}

		if cur == nil || !strings.EqualFold(section, "Peer") {
			continue
		}

		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch {
		case strings.EqualFold(k, "PublicKey"):
			cur.pubIdx = i
		case strings.EqualFold(k, "AllowedIPs"):
			cur.allowIdx = i
			cur.allowed = v
		case strings.EqualFold(k, "Endpoint"):
			cur.endpIdx = i
		case strings.EqualFold(k, "PersistentKeepalive"):
			cur.keepIdx = i
		}
	}

	if len(blocks) == 0 {
		return conf, false, fmt.Errorf("wg conf missing [Peer] section")
	}

	// Pick the peer that routes the address we care about.
	targetIdx := -1
	want := strings.TrimSpace(matchAllowedIPs)
	for i, b := range blocks {
		if allowedContains(b.allowed, want) {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		// Fallback: patch the first peer (common for arc-generated configs).
		targetIdx = 0
	}
	b := blocks[targetIdx]

	changed := false
	insertAt := b.startIdx + 1

	// Ensure PublicKey
	pubLine := "PublicKey = " + strings.TrimSpace(setPeerPublicKey)
	if b.pubIdx >= 0 {
		if strings.TrimSpace(lines[b.pubIdx]) != pubLine {
			lines[b.pubIdx] = pubLine
			changed = true
		}
	} else {
		lines = append(lines[:insertAt], append([]string{pubLine}, lines[insertAt:]...)...)
		changed = true
		// Adjust indices after insertion.
		if b.allowIdx >= insertAt {
			b.allowIdx++
		}
		if b.endpIdx >= insertAt {
			b.endpIdx++
		}
		if b.keepIdx >= insertAt {
			b.keepIdx++
		}
	}

	// Ensure AllowedIPs
	if want != "" {
		allowLine := "AllowedIPs = " + want
		if b.allowIdx >= 0 {
			if strings.TrimSpace(lines[b.allowIdx]) != allowLine {
				lines[b.allowIdx] = allowLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{allowLine}, lines[insertAt:]...)...)
			changed = true
			if b.endpIdx >= insertAt {
				b.endpIdx++
			}
			if b.keepIdx >= insertAt {
				b.keepIdx++
			}
		}
	}

	// Ensure Endpoint (client)
	if strings.TrimSpace(ensureEndpoint) != "" {
		endpLine := "Endpoint = " + strings.TrimSpace(ensureEndpoint)
		if b.endpIdx >= 0 {
			if strings.TrimSpace(lines[b.endpIdx]) != endpLine {
				lines[b.endpIdx] = endpLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{endpLine}, lines[insertAt:]...)...)
			changed = true
			if b.keepIdx >= insertAt {
				b.keepIdx++
			}
		}
	}

	// Ensure PersistentKeepalive (client)
	if strings.TrimSpace(ensureKeepalive) != "" {
		keepLine := "PersistentKeepalive = " + strings.TrimSpace(ensureKeepalive)
		if b.keepIdx >= 0 {
			if strings.TrimSpace(lines[b.keepIdx]) != keepLine {
				lines[b.keepIdx] = keepLine
				changed = true
			}
		} else {
			lines = append(lines[:insertAt], append([]string{keepLine}, lines[insertAt:]...)...)
			changed = true
		}
	}

	// Keep trailing newline behavior stable (arc-generated configs end with blank line).
	out := strings.Join(lines, "\n")
	return out, changed, nil
}
