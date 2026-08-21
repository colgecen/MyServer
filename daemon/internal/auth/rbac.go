package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

type Token struct {
	Value string `json:"value"`
	Role  Role   `json:"role"`
}

type Manager struct {
	mu     sync.Mutex
	tokens map[string]Role
}

func NewManager() *Manager { return &Manager{tokens: make(map[string]Role)} }

func (m *Manager) Issue(role Role) Token {
	b := make([]byte, 16)
	rand.Read(b)
	tok := "ms_" + hex.EncodeToString(b)
	m.mu.Lock()
	m.tokens[tok] = role
	m.mu.Unlock()
	return Token{Value: tok, Role: role}
}

func (m *Manager) Verify(token string) (Role, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.tokens[token]
	return r, ok
}

func (m *Manager) Can(role Role, action string) bool {
	switch action {
	case "exec":
		return role == RoleEditor || role == RoleAdmin
	case "admin":
		return role == RoleAdmin
	default:
		return true
	}
}
