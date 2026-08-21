package guardrail_test

import (
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/guardrail"
)

func TestSafeExec_ReadOnlyAutoExecute(t *testing.T) {
	se := guardrail.New(nil)
	out := se.Evaluate(guardrail.Request{ID: "r1", ClientID: "c1", Command: "ls -la"})
	if out.Decision != guardrail.DecisionExecute || out.Class.Level != guardrail.LevelReadOnly {
		t.Fatalf("got %+v", out)
	}
}

func TestSafeExec_BlacklistDenyAndAlert(t *testing.T) {
	se := guardrail.New(nil)
	out := se.Evaluate(guardrail.Request{ID: "r2", ClientID: "c1", Command: "rm -rf /"})
	if out.Decision != guardrail.DecisionDeny {
		t.Fatalf("decision=%s", out.Decision)
	}
	if !strings.Contains(out.Message, "SECURITY ALERT") {
		t.Fatalf("missing alert banner: %q", out.Message)
	}
}

func TestSafeExec_ConfirmationFlow(t *testing.T) {
	se := guardrail.New(nil)

	out := se.Evaluate(guardrail.Request{ID: "r3", ClientID: "c1", Command: "touch new.txt"})
	if out.Decision != guardrail.DecisionConfirm {
		t.Fatalf("expected confirm, got %s", out.Decision)
	}
	if se.PendingCount() != 1 {
		t.Fatalf("pending=%d", se.PendingCount())
	}

	req, class, err := se.Approve("r3", true)
	if err != nil {
		t.Fatal(err)
	}
	if req.Command != "touch new.txt" || class.Level != guardrail.LevelWorkspaceWrite {
		t.Fatalf("staged req mismatch: %+v %+v", req, class)
	}
	if se.PendingCount() != 0 {
		t.Fatal("pending not cleared")
	}

	// deny path
	_ = se.Evaluate(guardrail.Request{ID: "r4", ClientID: "c1", Command: "mkdir d"})
	_, _, err = se.Approve("r4", false)
	if err != nil {
		t.Fatal(err)
	}
	if se.PendingCount() != 0 {
		t.Fatal("deny did not clear pending")
	}

	// unknown id
	if _, _, err := se.Approve("nope", true); err == nil {
		t.Fatal("expected ErrNotPending")
	}

	// duplicate staging rejected
	_ = se.Evaluate(guardrail.Request{ID: "dup", ClientID: "c1", Command: "cp a b"})
	out2 := se.Evaluate(guardrail.Request{ID: "dup", ClientID: "c1", Command: "cp a b"})
	if out2.Decision != guardrail.DecisionDeny {
		t.Fatalf("duplicate id should be denied, got %s", out2.Decision)
	}
}

func TestSafeExec_RateLimitCooldown(t *testing.T) {
	cfg := guardrail.RateLimiterConfig{
		BurstSize:     3,
		BurstWindow:   time.Second,
		Cooldown:      50 * time.Millisecond,
		MaxDeniedRate: 100,
	}
	se := guardrail.NewWithRateLimit(cfg)

	allowed := 0
	for i := 0; i < 10; i++ {
		out := se.Evaluate(guardrail.Request{ID: string(rune('a' + i)), ClientID: "bursty", Command: "ls"})
		if out.Decision == guardrail.DecisionCooldown {
			break
		} else if out.Decision == guardrail.DecisionExecute {
			allowed++
			_, _, _ = se.Approve(string(rune('a'+i)), true) // drain pendings
		}
	}
	if allowed > cfg.BurstSize {
		t.Fatalf("allowed %d > burst %d", allowed, cfg.BurstSize)
	}

	time.Sleep(60 * time.Millisecond) // cooldown elapses
	out := se.Evaluate(guardrail.Request{ID: "z", ClientID: "bursty", Command: "ls"})
	if out.Decision != guardrail.DecisionExecute {
		t.Fatalf("post-cooldown decision=%s", out.Decision)
	}
	_, _, _ = se.Approve("z", true)
}

func TestSafeExec_AnomalyEscalationOnDenials(t *testing.T) {
	cfg := guardrail.DefaultRateLimit()
	cfg.MaxDeniedRate = 2
	se := guardrail.NewWithRateLimit(cfg)

	for i := 0; i < 2; i++ {
		out := se.Evaluate(guardrail.Request{ID: string(rune('a' + i)), ClientID: "attacker", Command: "mkfs.ext4 /dev/sda"})
		if out.Decision != guardrail.DecisionDeny {
			t.Fatalf("expected deny, got %s", out.Decision)
		}
	}
	// third attempt should hit escalated cooldown even for a benign command
	out := se.Evaluate(guardrail.Request{ID: "x", ClientID: "attacker", Command: "ls"})
	if out.Decision != guardrail.DecisionCooldown {
		t.Fatalf("anomaly cooldown not applied: %s", out.Decision)
	}
}

func TestSanitizeEnv_StripsInjectionVectors(t *testing.T) {
	env := guardrail.SanitizeEnv([]string{
		"PATH=/usr/bin",
		"LD_PRELOAD=/tmp/evil.so", // denied name -> dropped
		"MYAPP_MODE=fast",         // caller extra kept
	})

	joined := strings.Join(env, "\n")
	for _, banned := range []string{"LD_PRELOAD", "BASH_ENV", "NODE_OPTIONS", "PROMPT_COMMAND"} {
		if strings.Contains(joined, banned+"=") {
			t.Errorf("dangerous env var leaked: %s", banned)
		}
	}
	if !strings.Contains(joined, "PATH=") {
		t.Error("PATH missing from sanitized env")
	}
	if !strings.Contains(joined, "MYAPP_MODE=fast") {
		t.Error("caller extra dropped")
	}
}