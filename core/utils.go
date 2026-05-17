package core

import (
	"crypto/rand"
	"math/big"
)

// ZeroMemory는 매개변수로 받은 바이트 슬라이스의 모든 값을 0으로 덮어씁니다.
func ZeroMemory(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// GenerateRandomPassword는 암호학적으로 안전한 무작위 비밀번호를 생성합니다.
func GenerateRandomPassword(length int) ([]byte, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)

	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return nil, err
		}
		password[i] = charset[num.Int64()]
	}
	return password, nil
}
