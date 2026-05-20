package main

import (
	"fmt"
	"log"
	"os"
	"pass/core"
	"pass/storage"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

const vaultPath = "vault.dat"

// --- 1. Bubble Tea 애플리케이션 상태(Model) 정의 ---
type model struct {
	vault  *core.Vault
	key    []byte
	salt   []byte
	cursor int // 현재 선택된 리스트의 인덱스 (위/아래 방향키로 조절)
	err    error
}

// Init은 프로그램 시작 시 초기화할 명령을 정의합니다 (지금은 없음)
func (m model) Init() tea.Cmd {
	return nil
}

// --- 2. 이벤트 처리 (Update) ---
// 키보드 입력이나 기타 이벤트가 발생할 때마다 호출됩니다.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// 키보드 입력 이벤트
	case tea.KeyMsg:
		switch msg.String() {

		// 프로그램 종료 (Ctrl+C, q, esc)
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		// 위쪽 화살표
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// 아래쪽 화살표
		case "down", "j":
			if m.cursor < len(m.vault.Accounts)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

// --- 3. 화면 렌더링 (View) ---
// Model의 상태를 바탕으로 터미널에 그릴 문자열을 반환합니다.
func (m model) View() string {
	s := "🔐 Secure Password Manager\n\n"

	if len(m.vault.Accounts) == 0 {
		s += "저장된 계정이 없습니다. (새로운 계정을 추가하려면 기능을 구현해야 합니다)\n"
	} else {
		// 계정 목록 출력
		for i, acc := range m.vault.Accounts {
			// 커서가 위치한 항목은 "👉" 로 하이라이트 표시
			cursorStr := "  " // 기본 공백
			if m.cursor == i {
				cursorStr = "👉"
			}

			// 화면에는 비밀번호를 *** 로 마스킹 처리하여 출력
			s += fmt.Sprintf("%s %s | %s | ********\n", cursorStr, acc.Service, acc.Username)
		}
	}

	s += "\n[↑/↓: 이동] [q/esc: 종료]\n"
	return s
}

func main() {
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

	fmt.Print("🔑 마스터 비밀번호를 입력하세요: ")
	masterPassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		log.Fatalf("비밀번호 입력 에러: %v", err)
	}

	defer core.ZeroMemory(masterPassword)

	key := core.DeriveKey(masterPassword, salt)
	defer core.ZeroMemory(key)

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
		fmt.Println("\n[보안] 메모리 내 모든 평문 비밀번호 영역이 파기되었습니다.")
	}()

	// --- 4. TUI 프로그램 실행 ---
	// 작성한 모델을 기반으로 Bubble Tea 프로그램을 시작합니다.
	p := tea.NewProgram(model{
		vault:  vault,
		key:    key,
		salt:   salt,
		cursor: 0,
	})

	if _, err := p.Run(); err != nil {
		fmt.Printf("알 수 없는 에러가 발생했습니다: %v", err)
		os.Exit(1)
	}
}
