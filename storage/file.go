package storage

import (
	"encoding/json"
	"errors"
	"os"
	"pass/core"
)

// SaveVault는 Salt(16바이트)와 암호문을 결합하여 하나의 파일로 저장합니다.
func SaveVault(filePath string, vault *core.Vault, key, salt []byte) error {
	plaintext, err := json.Marshal(vault)
	if err != nil {
		return err
	}

	ciphertext, err := core.Encrypt(key, plaintext)
	if err != nil {
		return err
	}

	// [Salt 16바이트] + [AES-GCM 암호문] 형태로 결합
	finalData := append(salt, ciphertext...)
	return os.WriteFile(filePath, finalData, 0600)
}

// LoadSalt는 파일의 맨 앞 16바이트에서 Salt만 먼저 읽어옵니다.
func LoadSalt(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	salt := make([]byte, 16)
	_, err = file.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}

// LoadVault는 파일에서 Salt 이후의 암호문 영역만 복호화하여 구조체로 복원합니다.
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

	var vault core.Vault
	err = json.Unmarshal(plaintext, &vault)
	if err != nil {
		return nil, err
	}

	return &vault, nil
}
