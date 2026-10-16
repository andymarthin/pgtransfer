// Package sshx provides shared SSH helper utilities used across pgtransfer.
//
// It intentionally depends only on the standard library and
// golang.org/x/crypto so that it can be imported from any package without
// introducing import cycles.
package sshx

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Config mirrors the SSH portion of a connection profile. It is defined here
// (rather than importing internal/config) to avoid import cycles.
type Config struct {
	User       string
	Host       string
	Port       int
	KeyPath    string
	Passphrase string
	Password   string
	Timeout    int
}

// ReadPrivateKey reads an SSH private key file from disk.
func ReadPrivateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH private key: %w", err)
	}
	return data, nil
}

// BuildAuthMethods constructs the ordered list of SSH authentication methods.
// It always prefers the ssh-agent (when SSH_AUTH_SOCK is set), then an
// explicit private key, and finally password authentication.
func BuildAuthMethods(cfg Config) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod

	// Try ssh-agent first if SSH_AUTH_SOCK is available.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if agentConn, err := net.Dial("unix", sock); err == nil {
			auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
		}
	}

	// If a specific key path is provided, try to use it.
	if cfg.KeyPath != "" {
		key, err := ReadPrivateKey(os.ExpandEnv(cfg.KeyPath))
		if err != nil {
			return nil, err
		}

		var signer ssh.Signer
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	// Add password authentication if provided.
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}

	if len(auths) == 0 {
		return nil, fmt.Errorf("no valid SSH authentication method found")
	}

	return auths, nil
}
