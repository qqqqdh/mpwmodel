package core

// Account는 개별 서비스의 계정 정보를 담는 구조체
type Account struct {
	Service  string
	Username string
	Password []byte
}

// Vault는 암호화되어 파일로 저장될 전체 데이터 구조
type Vault struct {
	Accounts []Account
}
