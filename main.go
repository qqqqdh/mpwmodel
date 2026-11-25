package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"pass/core"
	"pass/storage"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

var vaultPath string

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		vaultPath = "vault.dat"
	} else {
		vaultPath = filepath.Join(homeDir, ".mpwmodel", "vault.dat")
		if err := os.MkdirAll(filepath.Dir(vaultPath), 0700); err != nil {
			log.Fatalf("저장 폴더 생성 실패: %v", err)
		}
	}

	fmt.Println("========================================")
	fmt.Println("    🔐 Secure Password Manager CLI")
	fmt.Println("========================================")

	var salt []byte
	var isNewVault bool

	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		isNewVault = true
		fmt.Println("[안내] 초기 저장소 파일이 없습니다. 새로운 마스터 비밀번호를 설정합니다.")
		newSalt, err := core.GenerateSalt()
		if err != nil {
			log.Fatalf("Salt 생성 실패: %v", err)
		}
		salt = newSalt
	} else {
		existingSalt, err := storage.LoadSalt(vaultPath)
		if err != nil {
			log.Fatalf("Salt 로드 실패: %v", err)
		}
		salt = existingSalt
	}
	var masterPassword []byte
	if isNewVault {
		for {
			fmt.Print("🔑 새로운 마스터 비밀번호를 입력하세요:")
			pwd1, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Fatalf("비밀번호 입력 에러: %v", err)
			}

			if len(pwd1) < 12 {
				fmt.Println("❌ 비밀번호는 최소 12자리 이상이어야 합니다. 다시 입력해주세요.")
				core.ZeroMemory(pwd1)
				continue
			}

			fmt.Print("🔑 마스터 비밀번호를 다시 입력하세요 (확인) ")
			pwd2, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Fatalf("비밀번호 입력 에러: %v", err)
			}

			if bytes.Equal(pwd1, pwd2) {
				masterPassword = pwd1
				core.ZeroMemory(pwd2)
				break
			} else {
				fmt.Println("❌ 비밀번호가 일치하지 않습니다. 다시 입력해주세요.")
				core.ZeroMemory(pwd1)
				core.ZeroMemory(pwd2)
			}
		}
	} else {
		fmt.Print("🔑 마스터 비밀번호를 입력하세요: ")
		var err error
		masterPassword, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			log.Fatalf("비밀번호 입력 에러: %v", err)
		}
	}
	defer core.ZeroMemory(masterPassword)
	key := core.DeriveKey(masterPassword, salt)
	defer core.ZeroMemory(key) // 앱 종료 시 파기

	var vault *core.Vault
	if isNewVault {
		vault = &core.Vault{Accounts: []core.Account{}}
	} else {
		loadedVault, err := storage.LoadVault(vaultPath, key)
		if err != nil {
			log.Fatalf("❌ 로그인 실패: 마스터 비밀번호가 틀렸거나 파일이 손상되었습니다. (%v)", err)
		}
		vault = loadedVault
	}

	defer func() {
		for i := range vault.Accounts {
			core.ZeroMemory(vault.Accounts[i].Password)
		}
		_ = clipboard.WriteAll("")
		fmt.Println("\n[보안] 메모리 내 모든 평문 비밀번호 파기 및 클립보드가 안전하게 초기화 되었습니다.")
	}()

	p := tea.NewProgram(initialModel(vault, key, salt))
	final, err := p.Run()
	if fm, ok := final.(model); ok {
		core.ZeroMemory(fm.key) // 마스터 비밀번호를 바꿨다면 새 키는 위 defer가 지우지 못함
	}
	if err != nil {
		fmt.Printf("알 수 없는 에러가 발생했습니다: %v", err)
		os.Exit(1)
	}
}
