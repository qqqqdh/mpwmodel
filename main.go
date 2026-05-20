package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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
)

type formMode int

const (
	modeAdd formMode = iota
	modeEdit
)

type model struct {
	vault   *core.Vault
	key     []byte
	salt    []byte
	cursor  int
	message string

	state      viewState
	mode       formMode
	inputs     []textinput.Model
	focusIndex int
}

func initialModel(v *core.Vault, k, s []byte) model {
	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "서비스명 (예: google)"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "아이디/이메일"

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "비밀번호 (빈칸 시 16자리 자동생성)"
	inputs[2].EchoMode = textinput.EchoPassword
	inputs[2].EchoCharacter = '*'

	return model{
		vault:  v,
		key:    k,
		salt:   s,
		state:  stateList,
		inputs: inputs,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.state == stateList {
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
						m.message = fmt.Sprintf("✅ '%s' 계정 비밀번호가 복사되었습니다!", acc.Service)
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
			}
		} else {

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
	if pwdInput == "" {
		newPwd, _ = core.GenerateRandomPassword(16)
	} else {
		newPwd = []byte(pwdInput)
	}

	switch m.mode {
	case modeAdd:
		m.vault.Accounts = append(m.vault.Accounts, core.Account{
			Service:  service,
			Username: username,
			Password: newPwd,
		})
		m.message = fmt.Sprintf("🎉 '%s' 계정이 성공적으로 추가되었습니다!", service)
	case modeEdit:
		core.ZeroMemory(m.vault.Accounts[m.cursor].Password)
		m.vault.Accounts[m.cursor].Service = service
		m.vault.Accounts[m.cursor].Username = username
		m.vault.Accounts[m.cursor].Password = newPwd
		m.message = fmt.Sprintf("✏️ '%s' 계정이 수정되었습니다!", service)
	}

	if err := storage.SaveVault(vaultPath, m.vault, m.key, m.salt); err != nil {
		m.message = "❌ 파일 저장 실패: " + err.Error()
	}

	m.state = stateList
	return m, nil
}

func (m model) View() string {
	s := "🔐 Secure Password Manager\n\n"

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
		s += "\n[↑/↓: 이동] [c: 복사] [a: 추가] [e: 수정] [q: 종료]\n"
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

	s += "\n[Tab/↓: 다음 칸] [Enter: 저장] [Esc: 취소]\n"
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

	p := tea.NewProgram(initialModel(vault, key, salt))
	if _, err := p.Run(); err != nil {
		fmt.Printf("알 수 없는 에러가 발생했습니다: %v", err)
		os.Exit(1)
	}
}
