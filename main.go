package main

import (
	"bytes"
	"fmt"
	"log"
	"pass/core"
)

func main() {
	fmt.Println("=== 🔐 [core] 패키지 단독 기능 테스트 ===")

	// 테스트용 마스터 비밀번호와 평문 데이터
	masterPassword := []byte("MinsuSuperSecret!!")
	originalPlaintext := []byte("my-naver-password-9999")

	// 1. Salt 생성 테스트
	salt, err := core.GenerateSalt()
	if err != nil {
		log.Fatalf("[실패] Salt 생성 중 에러 발생: %v", err)
	}
	fmt.Printf("[1] Salt 생성 성공: %x (길이: %d 바이트)\n", salt, len(salt))

	// 2. 마스터 키 파생 (Argon2id) 테스트
	key := core.DeriveKey(masterPassword, salt)
	fmt.Printf("[2] 암호화 키 생성 성공: %x (길이: %d 바이트)\n", key, len(key))

	// 3. AES-GCM 암호화 테스트
	ciphertext, err := core.Encrypt(key, originalPlaintext)
	if err != nil {
		log.Fatalf("[실패] 암호화 중 에러 발생: %v", err)
	}
	fmt.Println("[3] 암호화 성공")
	fmt.Printf("    - 원본 평문: %s\n", string(originalPlaintext))
	fmt.Printf("    - 암호화 결과(16진수): %x\n", ciphertext)

	// 4. AES-GCM 복호화 테스트 (올바른 키 사용)
	decryptedText, err := core.Decrypt(key, ciphertext)
	if err != nil {
		log.Fatalf("[실패] 복호화 중 에러 발생: %v", err)
	}
	fmt.Println("[4] 올바른 키로 복호화 성공")
	fmt.Printf("    - 복호화된 평문: %s\n", string(decryptedText))

	// 5. 무결성 검증 (데이터 일치 여부)
	if bytes.Equal(originalPlaintext, decryptedText) {
		fmt.Println("    => 🎉 [성공] 원본 데이터와 복호화된 데이터가 완벽히 일치합니다.")
	} else {
		fmt.Println("    => ❌ [실패] 복호화된 데이터가 원본과 다릅니다.")
	}

	// 6. 무결성 방어 테스트 (틀린 키 사용 시 복호화 실패 확인)
	fmt.Println("\n=== 🛡️ [core] 보안 및 무결성 방어 테스트 ===")
	wrongPassword := []byte("WrongPassword??")
	wrongKey := core.DeriveKey(wrongPassword, salt) // 틀린 비밀번호로 생성한 키

	_, err = core.Decrypt(wrongKey, ciphertext)
	if err != nil {
		fmt.Println("[5] 예상대로 틀린 키 복호화 차단 성공!")
		fmt.Printf("    - 차단 에러 메시지: %v\n", err)
		fmt.Println("    => 암호문이 조금이라도 변조되거나 키가 틀리면 복호화를 원천 차단합니다.")
	} else {
		fmt.Println("[❌ 경고] 보안 취약점 발견: 틀린 키인데도 복호화가 성공했습니다.")
	}
}
