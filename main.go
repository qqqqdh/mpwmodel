package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"pass/core"
	"pass/storage"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

const vaultPath = "vault.dat"

type viewState int

const (
	stateList viewState = iota
	stateForm
	stateDeleteConfirm
	stateChangeMaster
)

type formMode int

const (
	modeAdd formMode = iota
	modeEdit
)

type clearClipboardMsg struct{}

func clearClipboardCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		_ = clipboard.WriteAll("")
		return clearClipboardMsg{}
	})
}

type model struct {
	vault   *core.Vault
	key     []byte
	salt    []byte
	cursor  int
	message string

	state            viewState
	mode             formMode
	inputs           []textinput.Model
	focusIndex       int
	masterInputs     []textinput.Model // 마스터 비밀번호 변경 다중 입력 폼
	masterFocusIndex int
}

func initialModel(v *core.Vault, k, s []byte) model {
	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "서비스명 (예: google)"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "아이디/이메일"

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "비밀번호 (빈칸 시: 새 계정은 16자리 생성, 수정은 기존 유지)"
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].EchoCharacter = '*'

	// 마스터 비밀번호 변경용 인풋 초기화
	masterInputs := make([]textinput.Model, 3)
	placeholders := []string{"현재 마스터 비밀번호", "새로운 마스터 비밀번호", "새로운 마스터 비밀번호 확인"}

	for i := range masterInputs {
		masterInputs[i] = textinput.New()
		masterInputs[i].Placeholder = placeholders[i]
		masterInputs[i].EchoMode = textinput.EchoPassword
		masterInputs[i].EchoCharacter = '*'
	}

	return model{
		vault:        v,
		key:          k,
		salt:         s,
		state:        stateList,
		inputs:       inputs,
		masterInputs: masterInputs,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearClipboardMsg:
		m.message = "⏱️ (보안) 복사되었던 비밀번호가 클립보드에서 자동 삭제되었습니다."
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case stateList:
			switch msg.String() {
			case "q", "esc":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.message = ""
				}
			case "down", "j":
				if m.cursor < len(m.vault.Accounts)-1 {
					m.cursor++
					m.message = ""
				}
			case "c", "enter":
				if len(m.vault.Accounts) > 0 {
					acc := m.vault.Accounts[m.cursor]
					err := clipboard.WriteAll(string(acc.Password))
					if err != nil {
						m.message = "❌ 클립보드 복사 실패: " + err.Error()
					} else {
						m.message = fmt.Sprintf("✅ '%s' 계정 비밀번호가 복사되었습니다! (30초 후 삭제됨)", acc.Service)
						return m, clearClipboardCmd()
					}
				}

			case "a":
				m.state = stateForm
				m.mode = modeAdd
				m.resetForm()
				return m, textinput.Blink

			case "e":
				if len(m.vault.Accounts) > 0 {
					m.state = stateForm
					m.mode = modeEdit
					acc := m.vault.Accounts[m.cursor]

					m.resetForm()
					m.inputs[0].SetValue(acc.Service)
					m.inputs[1].SetValue(acc.Username)
					return m, textinput.Blink
				}

			case "d":
				if len(m.vault.Accounts) > 0 {
					m.state = stateDeleteConfirm
					m.message = fmt.Sprintf("⚠️ 정말 '%s' 계정을 삭제하시겠습니까? (y: 삭제 / n: 취소)", m.vault.Accounts[m.cursor].Service)
				}

			case "p":
				m.state = stateChangeMaster
				m.resetMasterForm()
				m.message = "🔑 마스터 비밀번호를 변경합니다."
				return m, textinput.Blink
			}

		case stateDeleteConfirm:
			switch strings.ToLower(msg.String()) {
			case "y":
				core.ZeroMemory(m.vault.Accounts[m.cursor].Password)
				m.vault.Accounts = append(m.vault.Accounts[:m.cursor], m.vault.Accounts[m.cursor+1:]...)
				if m.cursor >= len(m.vault.Accounts) && m.cursor > 0 {
					m.cursor--
				}
				if err := storage.SaveVault(vaultPath, m.vault, m.key, m.salt); err != nil {
					m.message = "❌ 파일 저장 실패: " + err.Error()
				} else {
					m.message = "🗑️ 계정이 성공적으로 삭제되었습니다."
				}
				m.state = stateList
			case "n", "esc":
				m.message = "❌ 삭제가 취소되었습니다."
				m.state = stateList
			}

		case stateChangeMaster:
			switch msg.String() {
			case "esc":
				m.state = stateList
				m.message = "입력이 취소되었습니다."
				return m, nil

			case "tab", "down":
				m.masterFocusIndex = (m.masterFocusIndex + 1) % len(m.masterInputs)
				return m.updateMasterFocus()

			case "shift+tab", "up":
				m.masterFocusIndex--
				if m.masterFocusIndex < 0 {
					m.masterFocusIndex = len(m.masterInputs) - 1
				}
				return m.updateMasterFocus()

			case "enter":
				if m.masterFocusIndex == len(m.masterInputs)-1 {
					return m.submitMasterForm()
				}
				m.masterFocusIndex++
				return m.updateMasterFocus()
			}

			cmds := make([]tea.Cmd, len(m.masterInputs))
			for i := range m.masterInputs {
				m.masterInputs[i], cmds[i] = m.masterInputs[i].Update(msg)
			}
			return m, tea.Batch(cmds...)

		case stateForm:
			switch msg.String() {
			case "esc":
				m.state = stateList
				m.message = "입력이 취소되었습니다."
				return m, nil

			case "tab", "down":
				m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
				return m.updateFocus()

			case "shift+tab", "up":
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}
				return m.updateFocus()

			case "ctrl+r":
				if m.mode == modeEdit {
					newPwd, _ := core.GenerateRandomPassword(16)
					m.inputs[2].SetValue(string(newPwd))

					_ = clipboard.WriteAll(string(newPwd))
					m.message = fmt.Sprintf("🔄 새 비밀번호(%s)가 폼에 생성되었고 클립보드에 복사되었습니다! (저장: Enter)", string(newPwd))
					return m, clearClipboardCmd()
				}

			case "enter":
				if m.focusIndex == len(m.inputs)-1 || m.mode == modeEdit {
					return m.submitForm()
				}
				m.focusIndex++
				return m.updateFocus()
			}

			cmd := m.updateInputs(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) updateFocus() (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := 0; i <= len(m.inputs)-1; i++ {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
			continue
		}
		m.inputs[i].Blur()
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updateMasterFocus() (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.masterInputs))
	for i := 0; i <= len(m.masterInputs)-1; i++ {
		if i == m.masterFocusIndex {
			cmds[i] = m.masterInputs[i].Focus()
			continue
		}
		m.masterInputs[i].Blur()
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *model) resetForm() {
	m.focusIndex = 0
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		if i == 0 {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *model) resetMasterForm() {
	m.masterFocusIndex = 0
	for i := range m.masterInputs {
		m.masterInputs[i].SetValue("")
		if i == 0 {
			m.masterInputs[i].Focus()
		} else {
			m.masterInputs[i].Blur()
		}
	}
}

func (m model) submitMasterForm() (tea.Model, tea.Cmd) {
	currentPwd := m.masterInputs[0].Value()
	newPwd := m.masterInputs[1].Value()
	confirmPwd := m.masterInputs[2].Value()

	if currentPwd == "" || newPwd == "" || confirmPwd == "" {
		m.message = "❌ 모든 항목을 입력해야 합니다."
		return m, nil
	}

	if newPwd != confirmPwd {
		m.message = "❌ 새 비밀번호가 서로 일치하지 않습니다."
		return m, nil
	}

	// 현재 비밀번호 검증
	currentBytes := []byte(currentPwd)
	defer core.ZeroMemory(currentBytes)
	tempKey := core.DeriveKey(currentBytes, m.salt)
	defer core.ZeroMemory(tempKey)

	if !bytes.Equal(tempKey, m.key) {
		m.message = "❌ 현재 마스터 비밀번호가 틀렸습니다."
		return m, nil
	}

	// 새 Salt 생성 및 오류 처리
	newSalt, err := core.GenerateSalt()
	if err != nil {
		m.message = "❌ 마스터 비밀번호 변경 중단 (Salt 생성 실패): " + err.Error()
		m.state = stateList
		return m, nil
	}

	// 새 비밀번호 메모리 파기 보장
	newMasterBytes := []byte(newPwd)
	defer core.ZeroMemory(newMasterBytes)

	newKey := core.DeriveKey(newMasterBytes, newSalt)

	if err := storage.SaveVault(vaultPath, m.vault, newKey, newSalt); err != nil {
		core.ZeroMemory(newKey)
		m.message = "❌ 마스터 비밀번호 변경 실패: " + err.Error()
	} else {
		core.ZeroMemory(m.key) // 기존 마스터 키 파기
		m.key = newKey
		m.salt = newSalt
		m.message = "✅ 마스터 비밀번호가 안전하게 변경(Key Rotation)되었습니다!"
	}

	m.resetMasterForm()
	m.state = stateList
	return m, nil
}

func (m model) submitForm() (tea.Model, tea.Cmd) {
	service := strings.TrimSpace(m.inputs[0].Value())
	username := strings.TrimSpace(m.inputs[1].Value())
	pwdInput := strings.TrimSpace(m.inputs[2].Value())

	if service == "" {
		m.message = "❌ 서비스명을 입력해야 합니다."
		m.state = stateList
		return m, nil
	}

	var newPwd []byte
	var isAutoGenerated bool

	if pwdInput == "" {
		if m.mode == modeEdit {
			newPwd = m.vault.Accounts[m.cursor].Password
		} else {
			newPwd, _ = core.GenerateRandomPassword(16)
			isAutoGenerated = true
		}
	} else {
		newPwd = []byte(pwdInput)
	}

	var cmd tea.Cmd

	switch m.mode {
	case modeAdd:
		m.vault.Accounts = append(m.vault.Accounts, core.Account{
			Service:  service,
			Username: username,
			Password: newPwd,
		})

		if isAutoGenerated {
			_ = clipboard.WriteAll(string(newPwd))
			m.message = fmt.Sprintf("🎉 '%s' 추가됨! (생성된 비밀번호: %s -> 클립보드 자동복사)", service, string(newPwd))
			cmd = clearClipboardCmd()
		} else {
			m.message = fmt.Sprintf("🎉 '%s' 계정이 성공적으로 추가되었습니다!", service)
		}

	case modeEdit:
		if string(m.vault.Accounts[m.cursor].Password) != string(newPwd) {
			core.ZeroMemory(m.vault.Accounts[m.cursor].Password)
			m.vault.Accounts[m.cursor].Password = newPwd
		}
		m.vault.Accounts[m.cursor].Service = service
		m.vault.Accounts[m.cursor].Username = username
		m.message = fmt.Sprintf("✏️ '%s' 계정이 수정되었습니다!", service)
	}

	if err := storage.SaveVault(vaultPath, m.vault, m.key, m.salt); err != nil {
		m.message = "❌ 파일 저장 실패: " + err.Error()
	}

	m.state = stateList
	return m, cmd
}

func (m model) View() string {
	s := "🔐 Secure Password Manager\n\n"

	if m.state == stateDeleteConfirm {
		return s + m.message + "\n"
	}

	if m.state == stateChangeMaster {
		s += m.message + "\n\n"
		for i := range m.masterInputs {
			s += m.masterInputs[i].View() + "\n"
		}
		s += "\n[Tab/↓: 다음 칸] [Enter: 변경] [Esc: 취소]\n"
		return s
	}

	if m.state == stateList {
		if len(m.vault.Accounts) == 0 {
			s += "저장된 계정이 없습니다. 'a'를 눌러 추가하세요.\n"
		} else {
			for i, acc := range m.vault.Accounts {
				cursorStr := "  "
				if m.cursor == i {
					cursorStr = "👉"
				}
				s += fmt.Sprintf("%s %-15s | %-15s | ********\n", cursorStr, acc.Service, acc.Username)
			}
		}

		if m.message != "" {
			s += fmt.Sprintf("\n%s\n", m.message)
		}
		s += "\n[↑/↓: 이동] [c: 복사] [a: 추가] [e: 수정] [d: 삭제] [p: 비밀번호 변경] [q: 종료]\n"
		return s
	}

	modeTitle := "새로운 계정 추가"
	if m.mode == modeEdit {
		modeTitle = "계정 수정"
	}
	s += fmt.Sprintf("--- %s ---\n\n", modeTitle)

	for i := range m.inputs {
		s += m.inputs[i].View() + "\n"
	}

	s += "\n[Tab/↓: 다음 칸] [Ctrl+R: 비밀번호 재생성] [Enter: 저장] [Esc: 취소]\n"
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
		fmt.Println("\n[보안] 메모리 내 모든 평문 비밀번호 영역이 파기되었습니다.")
	}()

	p := tea.NewProgram(initialModel(vault, key, salt))
	if _, err := p.Run(); err != nil {
		fmt.Printf("알 수 없는 에러가 발생했습니다: %v", err)
		os.Exit(1)
	}
}
