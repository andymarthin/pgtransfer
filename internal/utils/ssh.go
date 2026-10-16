package utils

import "github.com/andymarthin/pgtransfer/internal/sshx"

// ReadPrivateKey reads an SSH private key file from disk.
//
// Deprecated: use sshx.ReadPrivateKey directly.
func ReadPrivateKey(path string) ([]byte, error) {
	return sshx.ReadPrivateKey(path)
}
