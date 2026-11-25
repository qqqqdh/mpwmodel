package main

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"pass/core"
	"pass/storage"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type viewState int

const (
	stateList viewState = iota
	stateForm
	stateDeleteConfirm
	stateChangeMaster
	stateLocked
)

// idleTimeout 동안 키 입력이 없으면 자동 잠금됩니다.
const idleTimeout = 5 * time.Minute

type idleTickMsg struct{}

func idleTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return idleTickMsg{}
	})
}

type formMode int

const (
	modeAdd formMode = iota
	modeEdit
)

// seq는 어느 복사에 대한 타이머인지 구분해, 더 최근 복사를 일찍 지우지 않게 함
type clearClipboardMsg struct{ seq int }

// copySecret은 클립보드에 복사하고 30초 뒤 삭제 타이머를 겁니다.
func (m *model) copySecret(secret string) (tea.Cmd, error) {
	if err := clipboard.WriteAll(secret); err != nil {
		return nil, err
	}
	m.clipSeq++
	seq := m.clipSeq
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return clearClipboardMsg{seq: seq}
	}), nil
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
	lockInput        textinput.Model // 잠금 해제용 마스터 비밀번호 입력
	lastActivity     time.Time       // 자동 잠금 판단용 마지막 입력 시각
	clipSeq          int             // 마지막 클립보드 복사 번호
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

	lockInput := textinput.New()
	lockInput.Placeholder = "마스터 비밀번호"
	lockInput.EchoMode = textinput.EchoPassword
	lockInput.EchoCharacter = '*'

	return model{
		vault:        v,
		key:          k,
		salt:         s,
		state:        stateList,
		inputs:       inputs,
		masterInputs: masterInputs,
		lockInput:    lockInput,
		lastActivity: time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, idleTickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearClipboardMsg:
		if msg.seq != m.clipSeq {
			return m, nil // 이후에 다시 복사됨: 그 복사의 타이머가 지움
		}
		_ = clipboard.WriteAll("")
		m.message = "⏱️ (보안) 복사되었던 비밀번호가 클립보드에서 자동 삭제되었습니다."
		return m, nil

	case idleTickMsg:
		if m.state != stateLocked && time.Since(m.lastActivity) >= idleTimeout {
			m.state = stateLocked
			m.lockInput.SetValue("")
			m.lockInput.Focus()
			_ = clipboard.WriteAll("") // 잠금 시 클립보드도 함께 초기화
			m.message = "🔒 일정 시간 입력이 없어 자동 잠금되었습니다."
		}
		return m, idleTickCmd()

	case tea.KeyMsg:
		m.lastActivity = time.Now()

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case stateLocked:
			switch msg.String() {
			case "enter":
				ok := m.checkMaster(m.lockInput.Value())
				m.lockInput.SetValue("")

				if ok {
					m.state = stateList
					m.message = "🔓 잠금이 해제되었습니다."
				} else {
					m.message = "❌ 마스터 비밀번호가 틀렸습니다."
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.lockInput, cmd = m.lockInput.Update(msg)
			return m, cmd

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
					cmd, err := m.copySecret(string(acc.Password))
					if err != nil {
						m.message = "❌ 클립보드 복사 실패: " + err.Error()
					} else {
						m.message = fmt.Sprintf("✅ '%s' 계정 비밀번호가 복사되었습니다! (30초 후 삭제됨)", acc.Service)
						return m, cmd
					}
				}

			case "a":
				m.state = stateForm
				m.mode = modeAdd
				resetInputs(m.inputs, &m.focusIndex)
				return m, textinput.Blink

			case "e":
				if len(m.vault.Accounts) > 0 {
					m.state = stateForm
					m.mode = modeEdit
					acc := m.vault.Accounts[m.cursor]

					resetInputs(m.inputs, &m.focusIndex)
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
				resetInputs(m.masterInputs, &m.masterFocusIndex)
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
				return m, moveFocus(m.masterInputs, &m.masterFocusIndex, 1)

			case "shift+tab", "up":
				return m, moveFocus(m.masterInputs, &m.masterFocusIndex, -1)

			case "enter":
				if m.masterFocusIndex == len(m.masterInputs)-1 {
					return m.submitMasterForm()
				}
				return m, moveFocus(m.masterInputs, &m.masterFocusIndex, 1)
			}

			return m, updateInputs(m.masterInputs, msg)

		case stateForm:
			switch msg.String() {
			case "esc":
				m.state = stateList
				m.message = "입력이 취소되었습니다."
				return m, nil

			case "tab", "down":
				return m, moveFocus(m.inputs, &m.focusIndex, 1)

			case "shift+tab", "up":
				return m, moveFocus(m.inputs, &m.focusIndex, -1)

			case "ctrl+r":
				if m.mode == modeEdit {
					// 난수 생성 에러 검증 추가
					newPwd, err := core.GenerateRandomPassword(16)
					if err != nil {
						m.message = "❌ 새 비밀번호 생성 실패: " + err.Error()
						return m, nil
					}

					m.inputs[2].SetValue(string(newPwd))

					cmd, err := m.copySecret(string(newPwd))
					if err != nil {
						m.message = "🔄 새 비밀번호가 폼에 생성되었습니다 (클립보드 복사 실패: " + err.Error() + ") (저장: Enter)"
						return m, nil
					}
					m.message = "🔄 새 비밀번호가 폼에 생성되었고 클립보드에 복사되었습니다! (저장: Enter)"
					return m, cmd
				}

			case "enter":
				if m.focusIndex == len(m.inputs)-1 || m.mode == modeEdit {
					return m.submitForm()
				}
				return m, moveFocus(m.inputs, &m.focusIndex, 1)
			}

			return m, updateInputs(m.inputs, msg)
		}
	}
	return m, nil
}

// focusInput는 idx 칸에만 포커스를 주고 나머지는 해제합니다.
func focusInput(inputs []textinput.Model, idx int) tea.Cmd {
	cmds := make([]tea.Cmd, len(inputs))
	for i := range inputs {
		if i == idx {
			cmds[i] = inputs[i].Focus()
		} else {
			inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

// moveFocus는 Tab/Enter/화살표 키에 따라 포커스 인덱스를 순환 이동합니다.
func moveFocus(inputs []textinput.Model, idx *int, delta int) tea.Cmd {
	*idx = (*idx + delta + len(inputs)) % len(inputs)
	return focusInput(inputs, *idx)
}

func updateInputs(inputs []textinput.Model, msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(inputs))
	for i := range inputs {
		inputs[i], cmds[i] = inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func resetInputs(inputs []textinput.Model, idx *int) {
	for i := range inputs {
		inputs[i].SetValue("")
	}
	*idx = 0
	focusInput(inputs, 0)
}

// checkMaster는 입력된 비밀번호로 키를 유도해 현재 키와 상수 시간 비교합니다.
func (m model) checkMaster(pwd string) bool {
	pwdBytes := []byte(pwd)
	defer core.ZeroMemory(pwdBytes)
	tempKey := core.DeriveKey(pwdBytes, m.salt)
	defer core.ZeroMemory(tempKey)
	return subtle.ConstantTimeCompare(tempKey, m.key) == 1
}

func (m model) submitMasterForm() (tea.Model, tea.Cmd) {
	currentPwd := m.masterInputs[0].Value()
	newPwd := m.masterInputs[1].Value()
	confirmPwd := m.masterInputs[2].Value()

	if currentPwd == "" || newPwd == "" || confirmPwd == "" {
		m.message = "❌ 모든 항목을 입력해야 합니다."
		return m, nil
	}

	if len(newPwd) < 12 {
		m.message = "❌ 새 비밀번호는 최소 12자리 이상이어야 합니다."
		return m, nil
	}

	if newPwd != confirmPwd {
		m.message = "❌ 새 비밀번호가 서로 일치하지 않습니다."
		return m, nil
	}

	if !m.checkMaster(currentPwd) {
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

	resetInputs(m.masterInputs, &m.masterFocusIndex)
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
			var err error
			newPwd, err = core.GenerateRandomPassword(16)
			if err != nil {
				m.message = "❌ 비밀번호 생성 실패: " + err.Error()
				m.state = stateList
				return m, nil
			}
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
			var err error
			if cmd, err = m.copySecret(string(newPwd)); err != nil {
				m.message = fmt.Sprintf("🎉 '%s' 추가됨! (클립보드 복사 실패: %v)", service, err)
			} else {
				m.message = fmt.Sprintf("🎉 '%s' 추가됨! (생성된 비밀번호가 클립보드에 복사됨)", service)
			}
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

	if m.state == stateLocked {
		s += m.message + "\n\n"
		s += m.lockInput.View() + "\n"
		s += "\n[Enter: 잠금 해제] [Ctrl+C: 종료]\n"
		return s
	}

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

	// 폼 화면에서도 메시지를 정상적으로 렌더링하도록 추가
	if m.message != "" {
		s += fmt.Sprintf("\n%s\n", m.message)
	}

	s += "\n[Tab/↓: 다음 칸] [Ctrl+R: 비밀번호 재생성] [Enter: 저장] [Esc: 취소]\n"
	return s
}
