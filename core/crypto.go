package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// GenerateSalt는 랜덤한 16바이트 Salt를 생성
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveKey는 마스터 비밀번호와 Salt를 결합하여 32바이트 AES 키를 생성 (Argon2id 사용)
func DeriveKey(masterPassword, salt []byte) []byte {
	timeCost := uint32(1)
	memoryCost := uint32(64 * 1024) // 64MB
	threads := uint8(4)
	keyLen := uint32(32)

	return argon2.IDKey(masterPassword, salt, timeCost, memoryCost, threads, keyLen)
}

// Encrypt는 AES-GCM을 사용하여 평문 데이터를 암호화
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt는 암호문에서 Nonce를 분리한 뒤 평문으로 복호화
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
