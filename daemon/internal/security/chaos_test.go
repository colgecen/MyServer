package security_test

import (
	"sync"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/auth"
	"github.com/anomalyco/myserver/daemon/internal/security"
)

func TestChaos_Recovery(t *testing.T) {
	m := auth.NewManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := m.Issue(auth.RoleEditor)
			if _, ok := m.Verify(tok.Value); !ok {
				t.Errorf("verify failed")
			}
		}()
	}
	wg.Wait()
	// redact under race
	for i := 0; i < 100; i++ {
		_ = security.Redact("api_key=secret123")
	}
	// hmac chaos
	key := []byte("test-key")
	sig := security.SignEntry(key, map[string]string{"cmd": "ls"})
	if !security.VerifyEntry(key, map[string]string{"cmd": "ls"}, sig) {
		t.Fatal("hmac verify failed")
	}
	if security.VerifyEntry(key, map[string]string{"cmd": "rm"}, sig) {
		t.Fatal("hmac should fail for tampered")
	}
}
