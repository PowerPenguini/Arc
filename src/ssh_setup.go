package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const arcUser = "arc"

func parseSSHConnectTarget(target string) (user, host, addr string, err error) {
	parts := strings.SplitN(strings.TrimSpace(target), "@", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", "", fmt.Errorf("invalid target %q, expected user@host", target)
	}

	user = strings.TrimSpace(parts[0])
	host = strings.TrimSpace(parts[1])
	addr, err = normalizeSSHAddr(host)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid host %q: %w", host, err)
	}
	return user, host, addr, nil
}

func normalizeSSHAddr(host string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("host is empty")
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host, nil
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return net.JoinHostPort(strings.Trim(host, "[]"), "22"), nil
	}
	if strings.Count(host, ":") > 1 {
		return net.JoinHostPort(host, "22"), nil
	}
	return net.JoinHostPort(host, "22"), nil
}

func dialWithPassword(user, addr, password string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap auth failed for %s@%s: %w", user, addr, err)
	}
	return client, nil
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

func ensureLocalSSHKeyPair() error {
	sshDir := userSSHDir()
	privPath := filepath.Join(sshDir, "id_ed25519")
	pubPath := privPath + ".pub"

	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", sshDir, err)
	}

	privInfo, privErr := os.Stat(privPath)
	pubInfo, pubErr := os.Stat(pubPath)
	if privErr == nil && !privInfo.IsDir() && pubErr == nil && !pubInfo.IsDir() {
		return nil
	}

	if privErr == nil && !privInfo.IsDir() && os.IsNotExist(pubErr) {
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
