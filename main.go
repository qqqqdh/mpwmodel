package main

import (
	"fmt"
	"log"
	"os"
	"pass/core"
	"pass/storage"
)

func main() {
	// 테스트에 사용할 임시 파일 경로
	vaultPath := "vault.dat"

	// 테스트용 마스터 비밀번호 설정
	masterPassword := []byte("MySuperSecretPassword123!")

	fmt.Println("=== 1단계: 마스터 비밀번호 및 고유 Salt 생성 ===")
	// 실제 프로그램에서는 최초 가입 시 생성한 Salt를 파일에 저장해두고 계속 재사용해야 합니다.
	// 여기서는 테스트를 위해 즉석에서 생성합니다.
	salt, err := core.GenerateSalt()
	if err != nil {
		log.Fatalf("Salt 생성 실패: %v", err)
	}
	fmt.Printf("[Success] Salt 생성 완료 (길이: %d 바이트)\n\n", len(salt))

	fmt.Println("=== 2단계: 마스터 키 파생 (Argon2id) ===")
	key := core.DeriveKey(masterPassword, salt)
	fmt.Printf("[Success] 32바이트 AES 키 파생 완료\n\n")

	fmt.Println("=== 3단계: 테스트용 계정 데이터 생성 ===")
	mockVault := &core.Vault{
		Accounts: []core.Account{
			{Service: "google.com", Username: "minsu@gmail.com", Password: []byte("googlePlainPwd123")},
			{Service: "github.com", Username: "minsu-park", Password: []byte("gitSecret456")},
		},
	}
	fmt.Printf("저장할 계정 개수: %d개\n\n", len(mockVault.Accounts))

	fmt.Println("=== 4단계: 데이터 암호화 및 파일 저장 ===")
	err = storage.SaveVault(vaultPath, mockVault, key)
	if err != nil {
		log.Fatalf("Vault 저장 실패: %v", err)
	}
	fmt.Println("[Success] 암호화되어 'vault.dat' 파일로 저장되었습니다.")

	// 저장된 파일 내용이 평문이 아님을 확인하기 위해 크기 출력
	fileInfo, _ := os.Stat(vaultPath)
	fmt.Printf("생성된 파일 크기: %d 바이트\n\n", fileInfo.Size())

	fmt.Println("=== 5단계: 파일에서 복호화 및 데이터 로드 ===")
	loadedVault, err := storage.LoadVault(vaultPath, key)
	if err != nil {
		log.Fatalf("Vault 로드 실패: %v", err)
	}
	fmt.Println("[Success] 복호화 성공! 안전하게 데이터를 읽어왔습니다.")

	// 복호화된 데이터 출력하여 검증
	for _, acc := range loadedVault.Accounts {
		fmt.Printf("-> 서비스: %-12s | 아이디: %-15s | 비밀번호: %s\n",
			acc.Service, acc.Username, string(acc.Password))
	}
	fmt.Println()

	fmt.Println("=== 6단계: 의도적인 복호화 실패 테스트 (틀린 비밀번호) ===")
	wrongPassword := []byte("WrongPassword!!!")
	wrongKey := core.DeriveKey(wrongPassword, salt) // 틀린 비밀번호로 키 생성

	_, err = storage.LoadVault(vaultPath, wrongKey)
	if err != nil {
		fmt.Println("[Success] 예상대로 복호화 실패 (에러 메시지):", err)
		fmt.Println("-> AES-GCM 무결성 검증 덕분에 올바르지 않은 마스터 비밀번호를 완벽히 차단합니다.")
	} else {
		fmt.Println("[Fail] 무결성 검증 실패: 틀린 비밀번호로 데이터가 열렸습니다!")
	}

	// 테스트 종료 후 생성된 임시 파일 삭제
	os.Remove(vaultPath)
}
