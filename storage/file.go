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

	ciphertext, err := core.Encrypt(key, plaintext, salt)
	if err != nil {
		return err
	}

	// [Salt 16바이트] + [AES-GCM 암호문] 형태로 결합
	finalData := make([]byte, 0, len(salt)+len(ciphertext))
	finalData = append(finalData, salt...)
	finalData = append(finalData, ciphertext...)

	// 임시 파일에 쓰고 fsync로 디스크 반영을 보장한 뒤 원자적으로 교체
	tempPath := filePath + ".tmp"
	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(finalData); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// 기존 vault를 .bak으로 보존 (저장 실패/손상 시 복구용)
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Rename(filePath, filePath+".bak"); err != nil {
			return err
		}
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
	salt := fileData[:16]
	ciphertext := fileData[16:]

	plaintext, err := core.Decrypt(key, ciphertext, salt)
	isLegacy := false
	if err != nil {
		// 구버전 포맷(AAD 미적용) 호환: 성공 시 아래에서 새 포맷으로 재저장
		plaintext, err = core.Decrypt(key, ciphertext, nil)
		if err != nil {
			return nil, errors.New("마스터 비밀번호가 올바르지 않거나 vault 파일이 손상되었습니다")
		}
		isLegacy = true
	}

	defer core.ZeroMemory(plaintext)

	var vault core.Vault
	err = json.Unmarshal(plaintext, &vault)
	if err != nil {
		return nil, err
	}

	if isLegacy {
		if err := SaveVault(filePath, &vault, key, salt); err != nil {
			return nil, err
		}
	}

	return &vault, nil
}
