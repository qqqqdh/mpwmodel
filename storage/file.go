package storage

import (
	"encoding/json"
	"os"
	"pass/core"
)

func SaveVault(filePath string, vault *core.Vault, key []byte) error {
	plaintext, err := json.Marshal(vault)
	if err != nil {
		return err
	}

	ciphertext, err := core.Encrypt(key, plaintext)
	if err != nil {
		return err
	}

	err = os.WriteFile(filePath, ciphertext, 0600)
	if err != nil {
		return err
	}
	return nil
}

func LoadVault(filePath string, key []byte) (*core.Vault, error) {
	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	plaintext, err := core.Decrypt(key, ciphertext)
	if err != nil {
		return nil, err
	}

	var vault core.Vault
	err = json.Unmarshal(plaintext, &vault)
	if err != nil {
		return nil, err
	}

	return &vault, nil
}
