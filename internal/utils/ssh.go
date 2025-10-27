package utils

import (
	"fmt"
	"os"
)

func ReadPrivateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH private key: %w", err)
	}
	return data, nil
}
