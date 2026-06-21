package storage

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"pass/core"
)

// SaveVault는 Salt(16바이트)와 암호문을 결합하여 하나의 파일로 저장합니다.
func SaveVault(filePath string, vault *core.Vault, key, salt []byte) error {
	plaintext, err := json.Marshal(vault)
	if err != nil {
		return err
	}

	defer core.ZeroMemory(plaintext)

	ciphertext, err := core.Encrypt(key, plaintext)
	if err != nil {
		return err
	}

	// [Salt 16바이트] + [AES-GCM 암호문] 형태로 결합
	finalData := make([]byte, 0, len(salt)+len(ciphertext))
	finalData = append(finalData, salt...)
	finalData = append(finalData, ciphertext...)

	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, finalData, 0600); err != nil {
		return err
	}

	return os.Rename(tempPath, filePath)
}

func LoadSalt(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	salt := make([]byte, 16)
	if _, err := io.ReadFull(file, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func LoadVault(filePath string, key []byte) (*core.Vault, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if len(fileData) < 16 {
		return nil, errors.New("정상적인 vault 파일이 아닙니다")
	}

	// 맨 앞 16바이트(Salt)를 제외한 나머지가 암호문입니다.
	ciphertext := fileData[16:]

	plaintext, err := core.Decrypt(key, ciphertext)
	if err != nil {
		return nil, err
	}

	defer core.ZeroMemory(plaintext)

	var vault core.Vault
	err = json.Unmarshal(plaintext, &vault)
	if err != nil {
		return nil, err
	}

	return &vault, nil
}
