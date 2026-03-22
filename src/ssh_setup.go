package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const arcUser = "arc"

func parseSSHDeviceTarget(target string) (user, host, addr string, err error) {
	raw := strings.TrimSpace(target)
	if raw == "" {
		return "", "", "", fmt.Errorf("missing SSH target")
	}

	if strings.HasPrefix(strings.ToLower(raw), "ssh://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid SSH target %q: %w", target, err)
		}
		user = strings.TrimSpace(u.User.Username())
		if user == "" {
			return "", "", "", fmt.Errorf("invalid SSH target %q, expected ssh://user@host[:port]", target)
		}
		host = strings.TrimSpace(u.Hostname())
		if host == "" {
			return "", "", "", fmt.Errorf("invalid SSH target %q, missing host", target)
		}
		port := strings.TrimSpace(u.Port())
		if port == "" {
			port = "22"
		}
		return user, host, net.JoinHostPort(host, port), nil
	}

	parts := strings.SplitN(raw, "@", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", "", fmt.Errorf("invalid SSH target %q, expected ssh://user@host[:port] or user@host[:port]", target)
	}

	user = strings.TrimSpace(parts[0])
	host, addr, err = normalizeSSHTargetHost(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", "", "", fmt.Errorf("invalid host %q: %w", parts[1], err)
	}
	return user, host, addr, nil
}

func normalizeSSHTargetHost(raw string) (host, addr string, err error) {
	host = strings.TrimSpace(raw)
	if host == "" {
		return "", "", fmt.Errorf("host is empty")
	}

	if parsedHost, parsedPort, splitErr := net.SplitHostPort(host); splitErr == nil {
		if strings.TrimSpace(parsedHost) == "" {
			return "", "", fmt.Errorf("host is empty")
		}
		if strings.TrimSpace(parsedPort) == "" {
			return "", "", fmt.Errorf("port is empty")
		}
		return parsedHost, net.JoinHostPort(parsedHost, parsedPort), nil
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		stripped := strings.Trim(host, "[]")
		if stripped == "" {
			return "", "", fmt.Errorf("host is empty")
		}
		return stripped, net.JoinHostPort(stripped, "22"), nil
	}
	if strings.Count(host, ":") > 1 {
		return host, net.JoinHostPort(host, "22"), nil
	}
	return host, net.JoinHostPort(host, "22"), nil
}

func dialWithPassword(user, addr, password string) (*ssh.Client, error) {
	auth, authHint := bootstrapAuthMethods(password)
	if len(auth) == 0 {
		return nil, fmt.Errorf("bootstrap auth failed for %s@%s: no usable SSH key and no password provided", user, addr)
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap auth failed for %s@%s (%s): %w", user, addr, authHint, err)
	}
	return client, nil
}

func bootstrapAuthMethods(password string) ([]ssh.AuthMethod, string) {
	auth := make([]ssh.AuthMethod, 0, 2)
	hints := make([]string, 0, 2)

	signers := bootstrapKeySigners()
	if len(signers) > 0 {
		auth = append(auth, ssh.PublicKeys(signers...))
		hints = append(hints, "ssh-key")
	}

	if p := strings.TrimSpace(password); p != "" {
		auth = append(auth, ssh.Password(p))
		hints = append(hints, "password")
	}

	return auth, strings.Join(hints, "+")
}

func bootstrapKeySigners() []ssh.Signer {
	sshDir := userSSHDir()
	keyNames := []string{
		"id_ed25519",
		"id_ecdsa",
		"id_rsa",
		"id_dsa",
	}

	signers := make([]ssh.Signer, 0, len(keyNames))
	for _, name := range keyNames {
		signer, err := readPrivateKeySigner(filepath.Join(sshDir, name))
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	return signers
}

func canRunPrivileged(bootstrapUser string, client *ssh.Client, password string) (bool, error) {
	if bootstrapUser == "root" {
		return false, nil
	}
	if _, err := runRemoteCommand(client, "true", true, password); err != nil {
		return false, fmt.Errorf("bootstrap user %q is not root and sudo failed: %w", bootstrapUser, err)
	}
	return true, nil
}

func ensureArcUser(client *ssh.Client, useSudo bool, sudoPassword string) error {
	script, err := renderTemplateFile("templates/ssh_ensure_arc_user.sh.tmpl", map[string]string{
		"ArcUser": arcUser,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, useSudo, sudoPassword); err != nil {
		return fmt.Errorf("create user %q failed: %w", arcUser, err)
	}
	return nil
}

func ensureArcAuthorizedKey(client *ssh.Client, useSudo bool, sudoPassword, pubKeyLine string) error {
	if strings.TrimSpace(pubKeyLine) == "" {
		return fmt.Errorf("public key line is empty")
	}
	quotedKey := shSingleQuote(pubKeyLine)
	script, err := renderTemplateFile("templates/ssh_ensure_arc_authorized_key.sh.tmpl", map[string]string{
		"ArcUser":   arcUser,
		"QuotedKey": quotedKey,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, useSudo, sudoPassword); err != nil {
		return fmt.Errorf("install authorized_keys failed: %w", err)
	}
	return nil
}

func ensureArcSudoers(client *ssh.Client, useSudo bool, sudoPassword string) error {
	script, err := renderTemplateFile("templates/ssh_ensure_arc_sudoers.sh.tmpl", map[string]string{
		"ArcUser": arcUser,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, useSudo, sudoPassword); err != nil {
		return fmt.Errorf("install sudoers failed: %w", err)
	}
	return nil
}

func ensureArcHushLogin(client *ssh.Client, useSudo bool, sudoPassword string) error {
	script, err := renderTemplateFile("templates/ssh_ensure_arc_hushlogin.sh.tmpl", map[string]string{
		"ArcUser": arcUser,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, useSudo, sudoPassword); err != nil {
		return fmt.Errorf("install hushlogin failed: %w", err)
	}
	return nil
}

func verifyArcKeyLogin(host, addr string) error {
	client, err := dialArcWithKey(addr)
	if err != nil {
		return fmt.Errorf("arc key login failed for %s@%s: %w", arcUser, host, err)
	}
	defer client.Close()

	if _, err := runRemoteCommand(client, "true", false, ""); err != nil {
		return fmt.Errorf("arc login verification command failed: %w", err)
	}
	if _, err := runRemoteCommand(client, "sudo -n true", false, ""); err != nil {
		return fmt.Errorf("arc sudo verification failed: %w", err)
	}
	return nil
}

func dialArcWithKey(addr string) (*ssh.Client, error) {
	privPath := filepath.Join(userSSHDir(), "id_ed25519")
	signer, err := readPrivateKeySigner(privPath)
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            arcUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func ensureArcZshPrompt(addr string) error {
	client, err := dialArcWithKey(addr)
	if err != nil {
		return fmt.Errorf("cannot connect as %s: %w", arcUser, err)
	}
	defer client.Close()

	// Install/replace a dedicated ARC prompt block in ~/.zshrc.
	script, err := renderTemplateFile("templates/ssh_ensure_arc_zsh_prompt.sh.tmpl", map[string]string{
		"ArcPromptBlockRemote": arcPromptBlockRemote,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, false, ""); err != nil {
		return err
	}
	return nil
}

func ensureArcTmuxConfig(addr string) error {
	client, err := dialArcWithKey(addr)
	if err != nil {
		return fmt.Errorf("cannot connect as %s: %w", arcUser, err)
	}
	defer client.Close()

	script, err := renderTemplateFile("templates/ssh_ensure_arc_tmux_conf.sh.tmpl", map[string]string{
		"ArcTmuxBlockRemote": arcTmuxBlockRemote,
	})
	if err != nil {
		return err
	}

	if _, err := runRemoteCommand(client, script, false, ""); err != nil {
		return fmt.Errorf("install tmux config failed: %w", err)
	}
	return nil
}

func runRemoteCommand(client *ssh.Client, command string, useSudo bool, sudoPassword string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("cannot open ssh session: %w", err)
	}
	defer session.Close()

	remoteCmd := "/bin/sh -lc " + shSingleQuote(command)
	if useSudo {
		remoteCmd = "sudo -S -p '' -k " + remoteCmd
		session.Stdin = strings.NewReader(sudoPassword + "\n")
	}

	out, err := session.CombinedOutput(remoteCmd)
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func userSSHDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".ssh"
	}
	return filepath.Join(home, ".ssh")
}

func userSSHPublicKeyPath() string {
	return filepath.Join(userSSHDir(), "id_ed25519.pub")
}

func userMobileSSHPrivateKeyPath() string {
	return filepath.Join(userSSHDir(), "id_arc_mobile_rsa")
}

func userMobileSSHPublicKeyPath() string {
	return userMobileSSHPrivateKeyPath() + ".pub"
}

func ensureLocalSSHKeyPair() error {
	return ensureEd25519KeyPair(filepath.Join(userSSHDir(), "id_ed25519"))
}

func ensureLocalMobileSSHKeyPair() error {
	return ensureRSAKeyPair(userMobileSSHPrivateKeyPath())
}

func ensureEd25519KeyPair(privPath string) error {
	pubPath := privPath + ".pub"
	if err := ensureKeyPairDir(privPath); err != nil {
		return err
	}

	privInfo, privErr := os.Stat(privPath)
	pubInfo, pubErr := os.Stat(pubPath)
	if privErr == nil && !privInfo.IsDir() && pubErr == nil && !pubInfo.IsDir() {
		return nil
	}

	if privErr == nil && !privInfo.IsDir() && os.IsNotExist(pubErr) {
		return writePublicKeyFromPrivateKey(privPath, pubPath)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("cannot generate ed25519 key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("cannot encode private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return fmt.Errorf("cannot write %s: %w", privPath, err)
	}

	pubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("cannot encode public key: %w", err)
	}
	pubLine := string(ssh.MarshalAuthorizedKey(pubKey))
	if err := os.WriteFile(pubPath, []byte(pubLine), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", pubPath, err)
	}
	return nil
}

func ensureRSAKeyPair(privPath string) error {
	pubPath := privPath + ".pub"
	if err := ensureKeyPairDir(privPath); err != nil {
		return err
	}

	privInfo, privErr := os.Stat(privPath)
	pubInfo, pubErr := os.Stat(pubPath)
	if privErr == nil && !privInfo.IsDir() && pubErr == nil && !pubInfo.IsDir() {
		return nil
	}

	if privErr == nil && !privInfo.IsDir() && os.IsNotExist(pubErr) {
		return writePublicKeyFromPrivateKey(privPath, pubPath)
	}

	priv, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return fmt.Errorf("cannot generate rsa key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("cannot encode private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return fmt.Errorf("cannot write %s: %w", privPath, err)
	}

	pubKey, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return fmt.Errorf("cannot encode public key: %w", err)
	}
	pubLine := string(ssh.MarshalAuthorizedKey(pubKey))
	if err := os.WriteFile(pubPath, []byte(pubLine), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", pubPath, err)
	}
	return nil
}

func ensureKeyPairDir(privPath string) error {
	sshDir := filepath.Dir(privPath)
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", sshDir, err)
	}
	return nil
}

func writePublicKeyFromPrivateKey(privPath, pubPath string) error {
	signer, err := readPrivateKeySigner(privPath)
	if err != nil {
		return fmt.Errorf("cannot read existing private key: %w", err)
	}
	pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if pub == "" {
		return fmt.Errorf("derived public key is empty")
	}
	if err := os.WriteFile(pubPath, []byte(pub+"\n"), 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", pubPath, err)
	}
	return nil
}

func readPublicKeyLine(pubPath string) (string, error) {
	raw, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", pubPath, err)
	}
	key := strings.TrimSpace(string(raw))
	if key == "" {
		return "", fmt.Errorf("%s is empty", pubPath)
	}
	lines := strings.Split(key, "\n")
	return strings.TrimSpace(lines[0]), nil
}

func readPrivateKeySigner(privPath string) (ssh.Signer, error) {
	raw, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", privPath, err)
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", privPath, err)
	}
	return signer, nil
}

func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
